package lang

import (
	"bake/internal/values"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Address interface {
	GetName() string
	GetPath() cty.Path
	GetFilename() string
	Dependencies() ([]hcl.Traversal, hcl.Diagnostics)
}

type Action interface {
	Address
	values.Cty
	Apply() hcl.Diagnostics
}

type RawAddress interface {
	Address
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
		task, diagnostics := NewTask(n, ctx)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return []Action{task}, nil
	case DataLabel:
		data, diagnostics := NewData(n, ctx)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return []Action{data}, nil
	default:
		panic("missing label implementation " + n.block.Type)
	}
}
