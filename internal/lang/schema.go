package lang

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// labels
const (
	TaskLabel   = "task"
	DataLabel   = "data"
	LocalsLabel = "locals"
	NameLabel   = "name"
)

const (
	// LocalScope only for locals since the scope != label
	LocalScope = "local"
	// PathScope is automatically injected
	PathScope = "path"
)

// attributes
const (
	DependsOnAttr = "depends_on"
	CommandAttr   = "command"
	ForEachAttr   = "for_each" // todo
)

func RecipeSchema() *hcl.BodySchema {
	return &hcl.BodySchema{
		Attributes: nil,
		Blocks: []hcl.BlockHeaderSchema{{
			Type:       TaskLabel,
			LabelNames: []string{NameLabel},
		}, {
			Type:       DataLabel,
			LabelNames: []string{NameLabel},
		}, {
			Type:       LocalsLabel,
			LabelNames: []string{},
		}},
	}
}

type Local struct {
	addressAttribute
	value cty.Value
}

func (l Local) Apply() hcl.Diagnostics {
	return nil
}

func (l Local) CTY() cty.Value {
	return l.value
}
