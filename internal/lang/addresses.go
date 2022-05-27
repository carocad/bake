package lang

import (
	"bake/internal/lang/values"

	"github.com/hashicorp/hcl/v2"
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
	Plan() (bool, string, hcl.Diagnostics)
}

type RawAddress interface {
	Address
	Dependencies() ([]hcl.Traversal, hcl.Diagnostics)
	Decode(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics)
}

func NewPartialAddress(block *hcl.Block) ([]RawAddress, hcl.Diagnostics) {
	switch block.Type {
	case DataLabel:
		return []RawAddress{AddressBlock{
			Block: block,
		}}, nil
	case TaskLabel:
		diags := checkDescription(block)
		if diags.HasErrors() {
			return nil, diags
		}

		return []RawAddress{AddressBlock{
			Block: block,
		}}, nil
	case LocalsLabel:
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
	default:
		return nil, nil
	}
}

type AddressBlock struct {
	Block *hcl.Block
}

func (n AddressBlock) GetFilename() string {
	return n.Block.DefRange.Filename
}

func (n AddressBlock) GetName() string {
	return n.Block.Labels[0]
}

func (n AddressBlock) GetPath() cty.Path {
	if n.Block.Type == TaskLabel {
		return cty.GetAttrPath(n.GetName())
	}

	return cty.GetAttrPath(n.Block.Type).GetAttr(n.GetName())
}

func (n AddressBlock) Dependencies() ([]hcl.Traversal, hcl.Diagnostics) {
	attributes, diagnostics := n.Block.Body.JustAttributes()
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	deps := make([]hcl.Traversal, 0)
	for _, attribute := range attributes {
		deps = append(deps, attribute.Expr.Variables()...)
	}

	return deps, nil
}

func (n AddressBlock) Decode(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics) {
	switch n.Block.Type {
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
		panic("missing label implementation " + n.Block.Type)
	}
}
