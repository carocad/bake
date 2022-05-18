package lang

import (
	"bake/internal/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type Address interface {
	GetName() string
	GetPath() cty.Path
	GetFilename() string
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
			block: block,
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
			label: LocalScope,
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
	block *hcl.Block
}

func (n addressBlock) GetFilename() string {
	return n.block.DefRange.Filename
}

func (n addressBlock) GetName() string {
	return n.block.Labels[0]
}

func (n addressBlock) GetPath() cty.Path {
	if n.block.Type == TaskLabel {
		return cty.GetAttrPath(n.GetName())
	}

	return cty.GetAttrPath(n.block.Type).GetAttr(n.GetName())
}

func (n addressBlock) Dependencies() ([]hcl.Traversal, hcl.Diagnostics) {
	attributes, diagnostics := n.block.Body.JustAttributes()
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
	switch n.block.Type {
	case TaskLabel:
		task := &Task{addressBlock: n}
		diagnostics := gohcl.DecodeBody(n.block.Body, ctx, task)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		diagnostics = checkDependsOn(task.Remain)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return []Action{task}, nil
	case DataLabel:
		data := &Data{addressBlock: n}
		diagnostics := gohcl.DecodeBody(n.block.Body, ctx, data)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return []Action{data}, nil
	default:
		panic("missing label implementation " + n.block.Type)
	}
}
