package lang

import (
	"bake/internal/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Target struct { // todo: what is really optional?
	addressBlock
	Description string   `hcl:"description,optional"`
	Command     string   `hcl:"command,optional"`
	Creates     string   `hcl:"creates,optional"`
	Sources     []string `hcl:"sources,optional"`
	Filename    string   `hcl:"filename,optional"`
	Remain      hcl.Body `hcl:",remain"`
}

func (t Target) Apply() hcl.Diagnostics {
	// TODO implement me
	panic("implement me")
}

func (t Target) CTY() cty.Value {
	value := values.StructToCty(t)
	m := value.AsValueMap()
	m[NameLabel] = cty.StringVal(t.name)
	return cty.ObjectVal(m)
}
