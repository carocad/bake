package lang

import (
	"bake/internal/lang/config"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type addressAttribute struct {
	name  string
	label string
	expr  hcl.Expression
}

func (a addressAttribute) GetFilename() string {
	return a.expr.Range().Filename
}

func (a addressAttribute) GetName() string {
	return a.name
}

func (a addressAttribute) GetPath() cty.Path {
	return cty.GetAttrPath(a.label).GetAttr(a.name)
}

func (a addressAttribute) Dependencies() ([]hcl.Traversal, hcl.Diagnostics) {
	return a.expr.Variables(), nil
}

func (a addressAttribute) Decode(ctx *hcl.EvalContext) (Action, hcl.Diagnostics) {
	value, diagnostics := a.expr.Value(ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return Local{
		addressAttribute: a,
		value:            value,
	}, nil
}

type Local struct {
	addressAttribute
	value cty.Value
}

func (l Local) Apply(state *config.State) *sync.WaitGroup {
	return nil
}

func (l Local) CTY() cty.Value {
	return l.value
}

func (l Local) Hash() []config.Hash {
	return nil
}
