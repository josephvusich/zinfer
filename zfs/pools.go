package zfs

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
)

type PropertyLocation int

const (
	PropertyDefault PropertyLocation = iota
	PropertyLocal
	PropertyInherited
	PropertyReadonly
	PropertyTemporary
)

const (
	FeatureDisabled = "disabled"
	FeatureEnabled  = "enabled"
	FeatureActive   = "active"
)

type PropertySource struct {
	Location  PropertyLocation
	Parent    string
	Inherited *Property
}

type Property struct {
	Name       string
	localValue string
	Source     PropertySource
}

func (p *Property) Value() string {
	if p.Source.Location == PropertyInherited {
		return p.Source.Inherited.Value()
	}
	return p.localValue
}

func isParent(self, parent string) bool {
	return strings.HasPrefix(self, fmt.Sprintf("%s/", parent))
}

func (p *Property) statusOnly() bool {
	if p.Source.Location == PropertyTemporary {
		return true
	}
	if _, ok := statusProperties[p.Name]; ok {
		return true
	}
	_, ok := ignoreProperties[p.Name]
	return ok
}

func (p *Property) nonEncryptionInherit() bool {
	_, ok := encryptionInheritedProperties[p.Name]
	return !ok && !p.statusOnly()
}

func (p *Property) isFeature() bool {
	return strings.HasPrefix(p.Name, "feature@")
}

func (p *Property) flag(o string, opts *FlagOptions) []string {
	if p.statusOnly() || p.Source.Location == PropertyDefault || p.Source.Location == PropertyInherited {
		return nil
	}
	value := p.localValue
	if p.isFeature() {
		if value == FeatureDisabled || (opts.MinimalFeatures && value == FeatureEnabled) {
			return nil
		}
		if value == FeatureActive {
			value = FeatureEnabled
		}
	}
	return []string{fmt.Sprintf("-%s", o), fmt.Sprintf("%s=%s", p.Name, value)}
}

type Dataset struct {
	Name       string
	Properties map[string]*Property
}

func isRootDataset(name string) bool {
	return !strings.ContainsRune(name, '/')
}

type sortedProperties []*Property

func (s sortedProperties) Len() int {
	return len(s)
}

func (s sortedProperties) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s sortedProperties) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (d *Dataset) flags(o string) (flags []string) {
	var encryptedChild bool
	if er, ok := d.Properties[encryptionRoot]; ok && er.Value() != d.Name {
		encryptedChild = true
	}

	var sorted sortedProperties
	for _, p := range d.Properties {
		sorted = append(sorted, p)
	}
	sort.Sort(sorted)

	for _, p := range sorted {
		if encryptedChild {
			if _, ok := encryptionInheritedProperties[p.Name]; ok {
				if _, ok := encryptionLocalProperties[p.Name]; !ok {
					continue
				}
			}
		}
		flags = append(flags, p.flag(o, defaultFlagOpts)...)
	}

	return flags
}

type Pool struct {
	Name       string
	Properties map[string]*Property
	Datasets   struct {
		// Zero index is always the root dataset
		Ordered []*Dataset
		Index   map[string]*Dataset
	}
}

func (p *Pool) flags(opts *FlagOptions) (flags []string) {
	var sorted sortedProperties
	for _, p := range p.Properties {
		sorted = append(sorted, p)
	}
	sort.Sort(sorted)

	for _, p := range sorted {
		flags = append(flags, p.flag("o", opts)...)
	}

	return flags
}

func (p *Pool) addDataset(d *Dataset) error {
	if _, ok := p.Datasets.Index[d.Name]; ok {
		return fmt.Errorf("%s already contains a dataset named %s", p.Name, d.Name)
	}
	p.Datasets.Ordered = append(p.Datasets.Ordered, d)
	p.Datasets.Index[d.Name] = d
	return nil
}

// returns ancestors in ascending order: [0] is immediate parent
func (p *Pool) getAncestors(d *Dataset) (ancestors []*Dataset, err error) {
	if isRootDataset(d.Name) {
		return nil, nil
	}

	for name := path.Dir(d.Name); ; name = path.Dir(name) {
		parent, ok := p.Datasets.Index[name]
		if !ok {
			return nil, fmt.Errorf("unable to locate ancestor %s of %s", name, d.Name)
		}
		ancestors = append(ancestors, parent)
		if isRootDataset(name) {
			break
		}
	}

	return ancestors, nil
}

type FlagOptions struct {
	// Omit pool features that are "enabled" but not "active"
	MinimalFeatures bool
}

var defaultFlagOpts = &FlagOptions{}

func (p *Pool) CreatePoolCommand(opts *FlagOptions) (cmdline []string, err error) {
	if opts == nil {
		opts = defaultFlagOpts
	}

	root, ok := p.Datasets.Index[p.Name]
	if !ok {
		return nil, fmt.Errorf("missing root dataset: %s", p.Name)
	}

	cmdline = []string{"zpool", "create", "-d"}
	cmdline = append(cmdline, p.flags(opts)...)
	cmdline = append(cmdline, root.flags("O")...)
	cmdline = append(cmdline, p.Name)
	return cmdline, nil
}

func (p *Pool) CreateDatasetCommand(name string) (cmdline []string, err error) {
	set, ok := p.Datasets.Index[name]
	if !ok {
		return nil, fmt.Errorf("dataset %s not found in pool %s", name, p.Name)
	}

	cmdline = []string{"zfs", "create"}
	cmdline = append(cmdline, set.flags("o")...)
	cmdline = append(cmdline, set.Name)
	return cmdline, nil
}

var (
	header   = regexp.MustCompile(`^NAME\s+PROPERTY\s+VALUE\s+SOURCE$`)
	property = regexp.MustCompile(`^([^ ]+) +([^ ]+) +((?U).*) +(-|default|local|temporary|inherited from )([^ ]+)?$`)
)

func parseZpoolSource(name string, raw string) (*PropertySource, error) {
	switch raw {
	case "-":
		return &PropertySource{Location: PropertyReadonly}, nil
	case "default":
		return &PropertySource{Location: PropertyDefault}, nil
	case "local":
		return &PropertySource{Location: PropertyLocal}, nil
	default:
		return nil, fmt.Errorf("property source for %s is invalid: %s", name, raw)
	}
}

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
	case "temporary":
		return &PropertySource{Location: PropertyTemporary}, nil
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

	return nil, fmt.Errorf("property source for %s is invalid: %s", name, raw)
}

func zfsGetAllRaw() ([]byte, error) {
	return exec.Command(`zfs`, `get`, `all`).Output()
}

func zpoolGetAllRaw() ([]byte, error) {
	return exec.Command(`zpool`, `get`, `all`).Output()
}

func zpoolParse(b []byte) (map[string]map[string]*Property, error) {
	poolProps := make(map[string]map[string]*Property)

	lines := bytes.Split(b, []byte{'\n'})
	if !header.Match(lines[0]) {
		return nil, fmt.Errorf("unexpected header: %s", lines[0])
	}
	lines = lines[1:]

	poolName := ""
	for _, l := range lines {
		l = bytes.TrimSpace(l)
		if len(l) == 0 {
			continue
		}

		m := property.FindSubmatch(l)
		if m == nil {
			return nil, fmt.Errorf("unparseable input: %s", l)
		}

		nextName := string(m[1])
		if nextName != poolName {
			poolName = nextName
			if _, ok := poolProps[poolName]; ok {
				return nil, fmt.Errorf("duplicate zpool found: %s", poolName)
			}
			poolProps[poolName] = make(map[string]*Property)
		}

		propName := string(m[2])
		propSrc, err := parseZpoolSource(propName, string(m[4]))
		if err != nil {
			return nil, err
		}
		poolProps[poolName][propName] = &Property{
			Name:       propName,
			localValue: string(m[3]),
			Source:     *propSrc,
		}
	}
	return poolProps, nil
}

func ImportedPools() (map[string]*Pool, error) {
	b, err := zpoolGetAllRaw()
	if err != nil {
		return nil, err
	}

	poolProps, err := zpoolParse(b)
	if err != nil {
		return nil, err
	}

	b, err = zfsGetAllRaw()
	if err != nil {
		return nil, err
	}

	pools, err := parseGetAll(b, poolProps)
	if _, ok := err.(inputEOF); !ok {
		return nil, fmt.Errorf("error parsing pool properties: %w", err)
	}

	return pools, nil
}

func fixInheritance(pools map[string]*Pool) error {
	for _, pool := range pools {
		for _, set := range pool.Datasets.Ordered {
			if er, ok := set.Properties[encryptionRoot]; ok && er.Value() != "" && er.Value() != set.Name {
				rootSet, ok := pool.Datasets.Index[er.Value()]
				if !ok {
					return fmt.Errorf("%s encryptionroot %s not found", set.Name, er.Value())
				}

				if rootRoot, ok := rootSet.Properties[encryptionRoot]; !ok || rootRoot.Value() != er.Value() {
					return fmt.Errorf("encryptionroot %s of child %s is not self-rooted: %s != %s", rootSet.Name, set.Name, rootRoot.Value(), rootSet.Name)
				}

				// Non-parent encryptionroot is possible via cloning, but we don't set up inheritance here as command inference gets confusing
				if isParent(set.Name, rootSet.Name) {
					for propName := range encryptionInheritedProperties {
						rootProp, ok := rootSet.Properties[propName]
						if !ok {
							return fmt.Errorf("encrypted dataset %s is missing property: %s", rootSet.Name, propName)
						}

						selfProp, ok := set.Properties[propName]
						if !ok {
							return fmt.Errorf("encrypted dataset %s is missing property: %s", set.Name, propName)
						}

						if _, ok := encryptionLocalProperties[propName]; ok && rootProp.Value() != selfProp.Value() {
							continue
						}

						selfProp.Source = PropertySource{
							Location:  PropertyInherited,
							Parent:    rootSet.Name,
							Inherited: rootProp,
						}
					}
				}
			}

			if !isRootDataset(set.Name) {
				ancestors, err := pool.getAncestors(set)
				if err != nil {
					return err
				}

				for _, prop := range set.Properties {
					if prop.Source.Location == PropertyReadonly && prop.Source.Location != PropertyInherited && prop.nonEncryptionInherit() {
						for _, a := range ancestors {
							if parentProp, ok := a.Properties[prop.Name]; ok && prop.Source.Location != PropertyInherited {
								if parentProp.Value() == prop.Value() {
									prop.Source.Location = PropertyInherited
									prop.Source.Parent = a.Name
									prop.Source.Inherited = parentProp
								}
								break
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func parseGetAll(b []byte, poolProps map[string]map[string]*Property) (map[string]*Pool, error) {
	lines := bytes.Split(b, []byte{'\n'})
	if !header.Match(lines[0]) {
		return nil, fmt.Errorf("unexpected header: %s", lines[0])
	}
	lines = lines[1:]

	pools := make(map[string]*Pool)
	p := parser{lines: lines}
	for {
		pool, err := p.parsePool()
		if pool != nil {
			props, ok := poolProps[pool.Name]
			if !ok {
				return nil, fmt.Errorf("missing pool properties: %s", pool.Name)
			}
			pool.Properties = props
		}

		switch err.(type) {
		case nextPool:
			pools[pool.Name] = pool
		case inputEOF:
			pools[pool.Name] = pool
			if err := fixInheritance(pools); err != nil {
				return nil, err
			}
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
	if !isRootDataset(string(name)) {
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

		setName := string(m[1])
		if strings.ContainsRune(setName, '@') {
			continue
		}

		if i == 0 {
			if pool == nil {
				return nil, nextPool(setName)
			}

			if setName != set.Name {
				if setName != pool.Name && !isParent(setName, pool.Name) {
					return nil, nextPool(setName)
				}
				set.Name = setName
			} else {
				panic("blank set name")
			}
		} else {
			if setName != set.Name {
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
			Name:       name,
			localValue: value,
			Source:     *src,
		}
	}

	return set, inputEOF{}
}
