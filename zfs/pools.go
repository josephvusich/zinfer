package zfs

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type PropertyLocation int

const (
	PropertyDefault PropertyLocation = iota
	PropertyLocal
	PropertyInherited
	PropertyReadonly
)

type PropertySource struct {
	Location  PropertyLocation
	Parent    string
	Inherited *Property
}

type Property struct {
	Name   string
	value  string
	Source PropertySource
}

func (p *Property) Value() string {
	if p.Source.Location == PropertyInherited {
		return p.Source.Inherited.Value()
	}
	return p.value
}

func isParent(self, parent string) bool {
	return strings.HasPrefix(self, fmt.Sprintf("%s/", parent))
}

func (p *Property) flag(o string) (flag string, value string) {
	if _, ok := statusProperties[p.Name]; ok {
		return "", ""
	}
	if p.Source.Location == PropertyDefault || p.Source.Location == PropertyInherited {
		return "", ""
	}
	return fmt.Sprintf("-%s", o), fmt.Sprintf("%s=%s", p.Name, p.value)
}

type Dataset struct {
	Name       string
	Properties map[string]*Property
}

func (d *Dataset) flags(o string) (flags []string) {
	for _, p := range d.Properties {
		if f, v := p.flag(o); f != "" {
			flags = append(flags, f, v)
		}
	}
	return flags
}

type Pool struct {
	Name     string
	Datasets struct {
		// Zero index is always the root dataset
		Ordered []*Dataset
		Index   map[string]*Dataset
	}
}

func (p *Pool) addDataset(d *Dataset) error {
	if _, ok := p.Datasets.Index[d.Name]; ok {
		return fmt.Errorf("%s already contains a dataset named %s", p.Name, d.Name)
	}
	p.Datasets.Ordered = append(p.Datasets.Ordered, d)
	p.Datasets.Index[d.Name] = d
	return nil
}

func (d *Dataset) CreateCommand() (cmdline []string) {
	var o string
	if !strings.ContainsRune(d.Name, '/') {
		cmdline = []string{"zpool"}
		o = "O"
	} else {
		cmdline = []string{"zfs"}
		o = "o"
	}

	cmdline = append(cmdline, "create")
	cmdline = append(cmdline, d.flags(o)...)
	cmdline = append(cmdline, d.Name)
	return cmdline
}

var (
	header   = regexp.MustCompile(`^NAME\s+PROPERTY\s+VALUE\s+SOURCE$`)
	property = regexp.MustCompile(`^([^ ]+) +([^ ]+) +((?U).*) +(-|default|local|inherited from )([^ ]+)?$`)
)

func parseSource(name string, value string, raw string, parent string, pool *Pool) (*PropertySource, error) {
	if _, ok := statusProperties[name]; ok && raw != "-" {
		return nil, fmt.Errorf("property %s expected to be readonly", name)
	}

	switch raw {
	case "-":
		return &PropertySource{Location: PropertyReadonly}, nil
	case "default":
		return &PropertySource{Location: PropertyDefault}, nil
	case "local":
		return &PropertySource{Location: PropertyLocal}, nil
	case "inherited from ":
		if parent, ok := pool.Datasets.Index[parent]; ok {
			if prop, ok := parent.Properties[name]; ok {
				if value != prop.Value() {
					return nil, fmt.Errorf("inherited property %s does not match value on parent %s: %s != %s", name, parent.Name, value, prop.Value())
				}
				return &PropertySource{
					Location:  PropertyInherited,
					Parent:    parent.Name,
					Inherited: prop,
				}, nil
			}
			return nil, fmt.Errorf("parent %s does not contain property %s", parent.Name, name)
		}
		return nil, fmt.Errorf("parent %s not found", parent)
	}

	panic(fmt.Sprintf("property source for %s is invalid: %s", name, raw))
}

func zfsGetAllRaw() ([]byte, error) {
	return exec.Command(`zfs`, `get`, `all`).Output()
}

func ImportedPools() ([]*Pool, error) {
	b, err := zfsGetAllRaw()
	if err != nil {
		return nil, err
	}

	pools, err := parseGetAll(b)
	if _, ok := err.(inputEOF); ok {
		return pools, nil
	}
	return nil, fmt.Errorf("error parsing pool properties: %w", err)
}

func parseGetAll(b []byte) ([]*Pool, error) {
	lines := bytes.Split(b, []byte{'\n'})
	if !header.Match(lines[0]) {
		return nil, fmt.Errorf("unexpected header: %s", lines[0])
	}
	lines = lines[1:]

	var pools []*Pool
	p := parser{lines: lines}
	for {
		pool, err := p.parsePool()
		switch err.(type) {
		case nextPool:
			pools = append(pools, pool)
		case inputEOF:
			pools = append(pools, pool)
			return pools, err
		default:
			return nil, err
		}
	}
}

type parser struct {
	lines [][]byte
}

type nextPool string

func (n nextPool) Error() string {
	return string(n)
}

type inputEOF struct{}

func (e inputEOF) Error() string {
	return fmt.Sprintf("end of input")
}

func newPool(name nextPool) (*Pool, error) {
	if strings.ContainsRune(string(name), '/') {
		return nil, fmt.Errorf("first dataset in pool is not root: %s", name)
	}

	pool := &Pool{
		Name: string(name),
	}
	pool.Datasets.Index = make(map[string]*Dataset)
	return pool, nil
}

func (p *parser) parsePool() (*Pool, error) {
	_, err := p.parseDataset(nil)
	name, ok := err.(nextPool)
	if !ok {
		return nil, err
	}

	pool, err := newPool(name)
	if err != nil {
		return nil, err
	}

	for {
		set, err := p.parseDataset(pool)
		if err == nil {
			if e := pool.addDataset(set); e != nil {
				return nil, e
			}
			continue
		}

		switch err := err.(type) {
		case nextPool:
			return pool, err
		case inputEOF:
			if e := pool.addDataset(set); e != nil {
				return nil, e
			}
			return pool, err
		default:
			return nil, err
		}
	}
}

// if pool is nil, does not parse and returns nextPool
func (p *parser) parseDataset(pool *Pool) (*Dataset, error) {
	set := &Dataset{
		Properties: make(map[string]*Property),
	}

	for i, l := range p.lines {
		l = bytes.TrimSpace(l)
		if len(l) == 0 {
			continue
		}

		m := property.FindSubmatch(l)
		if m == nil {
			return nil, fmt.Errorf("unparseable input: %s", l)
		}

		if name := string(m[1]); i == 0 {
			if pool == nil {
				return nil, nextPool(name)
			}

			if name != set.Name {
				if name != pool.Name && !isParent(name, pool.Name) {
					return nil, nextPool(name)
				}
				set.Name = name
			} else {
				panic("blank set name")
			}
		} else {
			if name != set.Name {
				p.lines = p.lines[i:]
				return set, nil
			}
		}

		name := string(m[2])
		value := string(m[3])
		src, err := parseSource(name, value, string(m[4]), string(m[5]), pool)
		if err != nil {
			return nil, fmt.Errorf("%s %w", set.Name, err)
		}

		set.Properties[string(m[2])] = &Property{
			Name:   name,
			value:  value,
			Source: *src,
		}
	}

	return set, inputEOF{}
}
