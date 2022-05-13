package lang

import (
	"bake/internal/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Phony struct { // todo: what is really optional?
	addressBlock
	Description string `hcl:"description,optional"`
	Command     string `hcl:"command,optional"`
}

func (p Phony) Apply() hcl.Diagnostics {
	// TODO implement me
	panic("implement me")
}

func (p Phony) CTY() cty.Value {
	value := values.StructToCty(p)
	m := value.AsValueMap()
	m[NameLabel] = cty.StringVal(p.name)
	return cty.ObjectVal(m)
}
