package action

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Local struct {
	Name  string
	Value cty.Value
	expr  hcl.Expression
}

func NewLocals(attrs hcl.Attributes) []Address {
	locals := make([]Address, 0)
	for name, attr := range attrs {
		locals = append(locals, &Local{
			Name:  name,
			expr:  attr.Expr,
			Value: cty.NilVal,
		})
	}

	return locals
}

func (local Local) GetName() string {
	return local.Name
}

func (local Local) Path() cty.Path {
	return cty.GetAttrPath("local").GetAttr(local.Name)
}

func (local Local) Dependencies() []hcl.Traversal {
	return local.expr.Variables()
}

func (local *Local) Plan(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics) {
	value, diagnostics := local.expr.Value(ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	local.Value = value
	return nil, nil
}

func (local Local) CTY() cty.Value {
	return local.Value
}
