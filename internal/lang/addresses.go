package lang

import (
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"fmt"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func NewPartialAddress(block *hcl.Block) ([]config.RawAddress, hcl.Diagnostics) {
	switch block.Type {
	case schema.DataLabel:
		return []config.RawAddress{addressBlock{
			Block: block,
		}}, nil
	case schema.TaskLabel:
		diags := checkDescription(block)
		if diags.HasErrors() {
			return nil, diags
		}

		return []config.RawAddress{addressBlock{
			Block: block,
		}}, nil
	case schema.LocalsLabel:
		attributes, diagnostics := block.Body.JustAttributes()
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		addrs := make([]config.RawAddress, 0)
		for name, attribute := range attributes {
			addrs = append(addrs, Local{
				name: name,
				expr: attribute.Expr,
			})
		}

		return addrs, nil
	default:
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`Unknown block type "%s"`, block.Type),
			Detail:   "This is likely an internal bake error; unrelated to your recipes. Please file a bug report",
			Subject:  &block.TypeRange,
			Context:  &block.DefRange,
		}}
	}
}

func checkDescription(block *hcl.Block) hcl.Diagnostics {
	attrs, diags := block.Body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}

	for _, attr := range attrs {
		if attr.Name != schema.DescriptionAttr {
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

func (n addressBlock) GetPath() cty.Path {
	name := n.Block.Labels[0]
	if n.Block.Type == schema.TaskLabel {
		return cty.GetAttrPath(name)
	}

	return cty.GetAttrPath(n.Block.Type).GetAttr(name)
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

func (addr addressBlock) Decode(ctx *hcl.EvalContext) (config.Action, hcl.Diagnostics) {
	switch addr.Block.Type {
	case schema.TaskLabel:
		tasks, diagnostics := newTask(addr, ctx)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return tasks, nil
	case schema.DataLabel:
		data, diagnostics := newData(addr, ctx)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}

		return data, nil
	default:
		panic("missing label implementation " + addr.Block.Type)
	}
}

func applyIndexed[T config.RuntimeInstance](instances []T, state *config.State) *sync.WaitGroup {
	wait := &sync.WaitGroup{}
	for _, app := range instances {
		app := app
		wait.Add(1)
		state.Group.Go(func() error {
			defer wait.Done()

			diags := app.Apply(state)
			if diags.HasErrors() {
				return diags
			}

			return nil
		})
	}

	return wait
}

func applySingle(instance config.RuntimeInstance, state *config.State) *sync.WaitGroup {
	wait := &sync.WaitGroup{}
	wait.Add(1)
	state.Group.Go(func() error {
		defer wait.Done()

		diags := instance.Apply(state)
		if diags.HasErrors() {
			return diags
		}

		return nil
	})

	return wait
}
