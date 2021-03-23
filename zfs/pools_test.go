package zfs

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type expectSet struct {
	SetName    string
	Properties []Property
}

func TestParseGetAll(t *testing.T) {
	assert := require.New(t)

	poolInput := []byte(`NAME  PROPERTY  VALUE  SOURCE
foo  feature@d  disabled  local
foo  feature@e  enabled   local
foo  feature@a  active    local
bar  feature@d  disabled  local
bar  feature@e  enabled   local
bar  feature@a  active    local`)

	input := []byte(`NAME  PROPERTY  VALUE  SOURCE
foo          fizz            buzz        default
foo          mounted         no          -
foo/bar      buzz            fizz        -

fizz@buzz    nope            nah         -

bar          zzup            zzip        local
bar          xxup            xxip        -
bar/foo      mounted         yes         -
bar/foo      encryptionroot  bar/foo     -
bar/foo      encryption      foobar      -
bar/foo      keystatus       available   -
bar/foo      keylocation     prompt      local
bar/foo      keyformat       passphrase  -
bar/foo      pbkdf2iters     342K        -
bar/foo/bar  encryptionroot  bar/foo     -
bar/foo/bar  encryption      fizzybar    -
bar/foo/bar  keylocation     none        default
bar/foo/bar  keyformat       passphrase  -
bar/foo/bar  pbkdf2iters     342K        -
bar/foo/bar  keystatus       available   -
bar/foo/bar  readonly        on          temporary
bar/foo/bar  zzup            zzip        inherited from bar
bar/foo/bar  xxup            xxip        -`)

	expected := map[string][]expectSet{
		"foo": {
			{
				SetName: "foo",
				Properties: []Property{
					{
						Name:       "fizz",
						localValue: "buzz",
						Source: PropertySource{
							Location: PropertyDefault,
						},
					},
					{
						Name:       "mounted",
						localValue: "no",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
				},
			},
			{
				SetName: "foo/bar",
				Properties: []Property{
					{
						Name:       "buzz",
						localValue: "fizz",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
				},
			},
		},
		"bar": {
			{
				SetName: "bar",
				Properties: []Property{
					{
						Name:       "zzup",
						localValue: "zzip",
						Source: PropertySource{
							Location: PropertyLocal,
						},
					},
					{
						Name:       "xxup",
						localValue: "xxip",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
				},
			},
			{
				SetName: "bar/foo",
				Properties: []Property{
					{
						Name:       "mounted",
						localValue: "yes",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
					{
						Name:       "encryptionroot",
						localValue: "bar/foo",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
					{
						Name:       "encryption",
						localValue: "foobar",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
					{
						Name:       "keystatus",
						localValue: "available",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
					{
						Name:       "keylocation",
						localValue: "prompt",
						Source: PropertySource{
							Location: PropertyLocal,
						},
					},
					{
						Name:       "keyformat",
						localValue: "passphrase",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
					{
						Name:       "pbkdf2iters",
						localValue: "342K",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
				},
			},
			{
				SetName: "bar/foo/bar",
				Properties: []Property{
					{
						Name:       "zzup",
						localValue: "zzip",
						Source: PropertySource{
							Location: PropertyInherited,
							Parent:   "bar",
						},
					},
					{
						Name:       "xxup",
						localValue: "xxip",
						Source: PropertySource{
							Location: PropertyInherited,
							Parent:   "bar",
						},
					},
					{
						Name:       "readonly",
						localValue: "on",
						Source: PropertySource{
							Location: PropertyTemporary,
						},
					},
					{
						Name:       "encryptionroot",
						localValue: "bar/foo",
						Source: PropertySource{
							Location: PropertyInherited,
							Parent:   "bar/foo",
						},
					},
					{
						Name:       "encryption",
						localValue: "fizzybar",
						Source: PropertySource{
							Location: PropertyReadonly,
						},
					},
					{
						Name:       "keystatus",
						localValue: "available",
						Source: PropertySource{
							Location: PropertyInherited,
							Parent:   "bar/foo",
						},
					},
					{
						Name:       "keylocation",
						localValue: "prompt",
						Source: PropertySource{
							Location: PropertyInherited,
							Parent:   "bar/foo",
						},
					},
					{
						Name:       "keyformat",
						localValue: "passphrase",
						Source: PropertySource{
							Location: PropertyInherited,
							Parent:   "bar/foo",
						},
					},
					{
						Name:       "pbkdf2iters",
						localValue: "342K",
						Source: PropertySource{
							Location: PropertyInherited,
							Parent:   "bar/foo",
						},
					},
				},
			},
		},
	}

	dummyPools, err := zpoolParse(poolInput)
	assert.NoError(err)
	dummyPools["fizz"] = make(map[string]*Property)

	pools, err := parseGetAll(input, dummyPools)
	assert.EqualError(err, "end of input")

	assert.Len(pools, len(expected))

	for _, p := range pools {
		expected := expected[p.Name]
		assert.Len(p.Datasets.Ordered, len(expected))
		assert.Len(p.Datasets.Index, len(expected))

		for i, expect := range expected {
			set := p.Datasets.Ordered[i]

			assert.Equal(expect.SetName, set.Name)
			assert.Len(set.Properties, len(expect.Properties), "%s properties", set.Name)

			for _, p := range expect.Properties {
				assert.Contains(set.Properties, p.Name)
				assert.Equal(p.localValue, set.Properties[p.Name].Value(), "property %s on %s", p.Name, set.Name)
				assert.Equal(p.Source.Location, set.Properties[p.Name].Source.Location, "property %s source location on %s", p.Name, set.Name)
				assert.Equal(p.Source.Parent, set.Properties[p.Name].Source.Parent)
			}
		}
	}

	expectCmd := []string{
		`zpool create -d -o feature@a=enabled -o feature@e=enabled foo`,
		`zfs create -o buzz=fizz foo/bar`,
		`zpool create -d -o feature@a=enabled -O xxup=xxip -O zzup=zzip bar`,
		`zfs create -o encryption=foobar -o keyformat=passphrase -o keylocation=prompt -o pbkdf2iters=342K bar/foo`,
		`zfs create -o encryption=fizzybar bar/foo/bar`,
	}
	sort.Strings(expectCmd)

	var actual []string
	for _, pool := range pools {
		cmdline, err := pool.CreatePoolCommand(&FlagOptions{MinimalFeatures: pool.Name == "bar"})
		assert.NoError(err)
		actual = append(actual, strings.Join(cmdline, " "))
		for i, dataset := range pool.Datasets.Ordered {
			if i == 0 {
				continue
			}
			cmdline, err = pool.CreateDatasetCommand(dataset.Name)
			assert.NoError(err)
			actual = append(actual, strings.Join(cmdline, " "))
		}
	}
	sort.Strings(actual)
	assert.Equal(expectCmd, actual)
}

func TestIsParent(t *testing.T) {
	assert := require.New(t)

	assert.True(isParent("foo/foo/bar", "foo/foo"))
	assert.False(isParent("foo/foo/bar", "foo/bar"))
}

func TestParseFailures(t *testing.T) {
	assert := require.New(t)

	cases := map[string]string{
		"unparseable input: xyz": `NAME  PROPERTY  VALUE  SOURCE
xyz`,
		"unexpected header: foo": `foo`,
		"foo property mounted expected to be readonly": `NAME  PROPERTY  VALUE  SOURCE
foo  mounted  yes  default`,
		"foo already contains a dataset named foo": `NAME  PROPERTY  VALUE  SOURCE
foo      mounted  yes  -
foo/bar  mounted  yes  -
foo/foo  mounted  yes  -
foo  mounted  yes  -`,
		"bar already contains a dataset named bar": `NAME  PROPERTY  VALUE  SOURCE
bar      mounted  yes  -
bar/bar  mounted  yes  -
bar      mounted  yes  -
bar/foo  mounted  yes  -`,
		"foo/bar inherited property fizz does not match value on parent foo: fuzz != buzz": `NAME  PROPERTY  VALUE  SOURCE
foo      fizz  buzz   local
foo/bar  fizz  fuzz   inherited from foo`,
		"foo/bar parent foo does not contain property buzz": `NAME  PROPERTY  VALUE  SOURCE
foo      fizz  buzz   local
foo/bar  buzz  fuzz   inherited from foo`,
		"foo parent bar not found": `NAME  PROPERTY  VALUE  SOURCE
foo      fizz  buzz   inherited from bar`,
		"first dataset in pool is not root: foo/bar": `NAME  PROPERTY  VALUE  SOURCE
foo/bar  fizz  buzz   -`,
		"foo/bar encryptionroot bar not found": `NAME  PROPERTY  VALUE  SOURCE
foo      fizz            buzz   -
foo/bar  encryptionroot  bar    -`,
		"encryptionroot foo/bar of child foo is not self-rooted: bar != foo/bar": `NAME  PROPERTY  VALUE  SOURCE
foo      encryptionroot  foo/bar  -
foo/bar  encryptionroot  bar      -`,
	}

	dummyPools := map[string]map[string]*Property{
		"xyz": make(map[string]*Property),
		"foo": make(map[string]*Property),
		"bar": make(map[string]*Property),
	}
	for out, in := range cases {
		_, err := parseGetAll([]byte(in), dummyPools)
		assert.EqualError(err, out)
	}
}
