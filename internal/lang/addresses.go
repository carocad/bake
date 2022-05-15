package lang

import (
	"bake/internal/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type Address interface {
	GetName() string
	Path() cty.Path
}

type Action interface {
	Address
	values.Cty
	Apply() hcl.Diagnostics
}

type RawAddress interface {
	Address
	Dependencies() ([]hcl.Traversal, hcl.Diagnostics)
	Decode(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics)
}

func NewPartialAddress(block *hcl.Block) ([]RawAddress, hcl.Diagnostics) {
	if block.Type != LocalsLabel {
		return []RawAddress{addressBlock{
			name:  block.Labels[0],
			label: block.Type,
			body:  block.Body,
		}}, nil
	}

	attributes, diagnostics := block.Body.JustAttributes()
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	addrs := make([]RawAddress, 0)
	for name, attribute := range attributes {
		addrs = append(addrs, addressAttribute{
			name:  name,
			label: block.Type,
			expr:  attribute.Expr,
		})
	}

	return addrs, nil
}

type addressAttribute struct {
	name  string
	label string
	expr  hcl.Expression
}

func (a addressAttribute) GetName() string {
	return a.name
}

func (a addressAttribute) Path() cty.Path {
	return cty.GetAttrPath(LocalScope).GetAttr(a.name)
}

func (a addressAttribute) Dependencies() ([]hcl.Traversal, hcl.Diagnostics) {
	return a.expr.Variables(), nil
}

func (a addressAttribute) Decode(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics) {
	value, diagnostics := a.expr.Value(ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return []Action{
		Local{
			addressAttribute: a,
			value:            value,
		},
	}, nil
}

type addressBlock struct {
	name  string
	label string
	body  hcl.Body
}

func (n addressBlock) GetName() string {
	return n.name
}

func (n addressBlock) Path() cty.Path {
	if n.label == TaskLabel {
		return cty.GetAttrPath(n.name)
	}

	return cty.GetAttrPath(n.label).GetAttr(n.name)
}

func (n addressBlock) Dependencies() ([]hcl.Traversal, hcl.Diagnostics) {
	attributes, diagnostics := n.body.JustAttributes()
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	deps := make([]hcl.Traversal, 0)
	for _, attribute := range attributes {
		deps = append(deps, attribute.Expr.Variables()...)
	}

	return deps, nil
}

func (n addressBlock) Decode(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics) {
	switch n.label {
	case TaskLabel:
		target := Task{addressBlock: n}
		diagnostics := gohcl.DecodeBody(n.body, ctx, &target)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		diagnostics = checkDependsOn(target.Remain)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return []Action{target}, nil
	case DataLabel:
		data := Data{addressBlock: n}
		diagnostics := gohcl.DecodeBody(n.body, ctx, &data)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		diagnostics = checkDependsOn(data.Remain)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		// we need to refresh before the next actions are loaded since
		// they depend on the data values
		diagnostics = data.Refresh()
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return []Action{data}, nil
	default:
		panic("missing label implementation " + n.label)
	}
}