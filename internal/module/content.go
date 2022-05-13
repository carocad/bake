package module

import (
	"path/filepath"

	"bake/internal/lang"
	"bake/internal/module/action"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func (module *Module) GetContent(file *hcl.File, filename string) ([]action.Address, hcl.Diagnostics) {
	content, diags := file.Body.Content(lang.RecipeSchema())
	if diags.HasErrors() {
		return nil, diags
	}

	addrs := make([]action.Address, 0)
	for _, block := range content.Blocks {
		switch block.Type {
		case lang.PhonyLabel:
			act, diags := action.NewPhony(block, nil)
			if diags.HasErrors() {
				return nil, diags
			}
			addrs = append(addrs, act)
		case lang.TargetLabel:
			act, diags := action.NewTarget(block, nil)
			if diags.HasErrors() {
				return nil, diags
			}
			addrs = append(addrs, act)
		case lang.LocalsLabel:
			attributes, diags := block.Body.JustAttributes()
			if diags.HasErrors() {
				return nil, diags
			}
			locals := action.NewLocals(attributes)
			addrs = append(addrs, locals...)
		}
	}

	return addrs, nil
}

func (module Module) currentContext(filename string, fileAddrs FileMapping) (*hcl.EvalContext, hcl.Diagnostics) {
	variables := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(module.cwd),
			"module":  cty.StringVal(filepath.Join(module.cwd, filepath.Dir(filename))),
			"current": cty.StringVal(filepath.Join(module.cwd, filename)),
		}),
	}

	phony := map[string]cty.Value{}
	local := map[string]cty.Value{}
	for _, addresses := range fileAddrs {
		for _, act := range addresses {
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
				variables[name] = value
			}
		}
	}

	variables[lang.PhonyLabel] = cty.ObjectVal(phony)
	variables[lang.LocalScope] = cty.ObjectVal(local)
	return &hcl.EvalContext{
		Variables: variables,
		Functions: lang.Functions(),
	}, nil
}
