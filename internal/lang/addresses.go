package lang

import (
	"bake/internal/lang/schema"
	"bake/internal/lang/values"

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
	Apply(config State) hcl.Diagnostics
}

type RawAddress interface {
	Address
	Dependencies() ([]hcl.Traversal, hcl.Diagnostics)
	Decode(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics)
}

func AddressToString[T Address](addr T) string {
	return schema.PathString(addr.GetPath())
}

func NewPartialAddress(block *hcl.Block) ([]RawAddress, hcl.Diagnostics) {
	switch block.Type {
	case schema.DataLabel:
		return []RawAddress{addressBlock{
			Block: block,
		}}, nil
	case schema.TaskLabel:
		diags := checkDescription(block)
		if diags.HasErrors() {
			return nil, diags
		}

		return []RawAddress{addressBlock{
			Block: block,
		}}, nil
	case schema.LocalsLabel:
		attributes, diagnostics := block.Body.JustAttributes()
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		addrs := make([]RawAddress, 0)
		for name, attribute := range attributes {
			addrs = append(addrs, addressAttribute{
				name:  name,
				label: schema.LocalScope,
				expr:  attribute.Expr,
			})
		}

		return addrs, nil
	default:
		return nil, nil
	}
}

func checkDescription(block *hcl.Block) hcl.Diagnostics {
	attrs, diags := block.Body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}

	for _, attr := range attrs {
		if attr.Name != schema.DescripionAttr {
			continue
		}

		_, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return diags
		}
	}

	return nil
}

type addressBlock struct {
	Block *hcl.Block
}

func (n addressBlock) GetFilename() string {
	return n.Block.DefRange.Filename
}

func (n addressBlock) GetName() string {
	return n.Block.Labels[0]
}

func (n addressBlock) GetPath() cty.Path {
	if n.Block.Type == schema.TaskLabel {
		return cty.GetAttrPath(n.GetName())
	}

	return cty.GetAttrPath(n.Block.Type).GetAttr(n.GetName())
}

func (n addressBlock) Dependencies() ([]hcl.Traversal, hcl.Diagnostics) {
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

func (n addressBlock) Decode(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics) {
	switch n.Block.Type {
	case schema.TaskLabel:
		task, diagnostics := NewTask(n, ctx)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return []Action{task}, nil
	case schema.DataLabel:
		data, diagnostics := NewData(n, ctx)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return []Action{data}, nil
	default:
		panic("missing label implementation " + n.Block.Type)
	}
}

type CliCommand struct{ Name, Description string }

func GetPublicTasks(addrs []RawAddress) []CliCommand {
	commands := make([]CliCommand, 0)
	for _, addr := range addrs {
		if schema.IsKnownPrefix(addr.GetPath()) {
			continue
		}

		// can only be task block
		block, ok := addr.(addressBlock)
		if !ok {
			continue
		}

		attrs, diags := block.Block.Body.JustAttributes()
		if diags.HasErrors() {
			continue
		}

		attr, ok := attrs[schema.DescripionAttr]
		if !ok {
			continue
		}

		var description string
		diags = gohcl.DecodeExpression(attr.Expr, nil, &description)
		if diags.HasErrors() {
			continue
		}

		commands = append(commands, CliCommand{addr.GetName(), description})
	}

	return commands
}
