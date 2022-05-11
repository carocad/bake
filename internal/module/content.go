package module

import (
	"path/filepath"

	"bake/internal/lang"
	"bake/internal/module/action"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func (module *Module) GetContent(file *hcl.File, filename string) hcl.Diagnostics {
	content, diags := file.Body.Content(lang.RecipeSchema())
	if diags.HasErrors() {
		return diags
	}

	context, diags := module.parsingContext(filename)
	if diags.HasErrors() {
		return diags
	}

	for _, block := range content.Blocks {
		switch block.Type {
		case lang.PhonyLabel:
			act, diags := action.NewPhony(block, context)
			if diags.HasErrors() {
				return diags
			}
			module.addresses = append(module.addresses, act)
		case lang.TargetLabel:
			act, diags := action.NewTarget(block, context)
			if diags.HasErrors() {
				return diags
			}
			module.addresses = append(module.addresses, act)
		case lang.LocalsLabel:
			attributes, diags := block.Body.JustAttributes()
			if diags.HasErrors() {
				return diags
			}
			locals := action.NewLocals(attributes)
			module.addresses = append(module.addresses, locals...)
		}
	}

	return nil
}

func (module Module) parsingContext(filename string) (*hcl.EvalContext, hcl.Diagnostics) {
	ctx := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(module.cwd),
			"module":  cty.StringVal(filepath.Join(module.cwd, filepath.Dir(filename))),
			"current": cty.StringVal(filepath.Join(module.cwd, filename)),
		}),
	}

	return &hcl.EvalContext{
		Variables: ctx,
		Functions: lang.Functions(),
	}, nil
}

func (module Module) currentContext() (*hcl.EvalContext, hcl.Diagnostics) {
	ctx := map[string]cty.Value{}

	phony := map[string]cty.Value{}
	local := map[string]cty.Value{}
	for _, act := range module.addresses {
		name := act.GetName()
		path := act.Path()
		value := act.CTY()
		switch {
		case path.HasPrefix(phonyPrefix):
			phony[name] = value
		case path.HasPrefix(localPrefix):
			local[name] = value
		default:
			// only targets for now !!
			ctx[name] = value
		}
	}

	ctx[lang.PhonyLabel] = cty.ObjectVal(phony)
	ctx[lang.LocalScope] = cty.ObjectVal(local)
	return &hcl.EvalContext{
		Variables: ctx,
		Functions: lang.Functions(),
	}, nil
}
