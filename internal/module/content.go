package module

import (
	"path/filepath"

	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func (module *Module) GetContent(file *hcl.File) ([]lang.RawAddress, hcl.Diagnostics) {
	content, diags := file.Body.Content(lang.RecipeSchema())
	if diags.HasErrors() {
		return nil, diags
	}

	addrs := make([]lang.RawAddress, 0)
	for _, block := range content.Blocks {
		address, diagnostics := lang.NewPartialAddress(block)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}
		addrs = append(addrs, address...)
	}

	return addrs, nil
}

func (module Module) currentContext(filename string, actions []lang.Action) (*hcl.EvalContext, hcl.Diagnostics) {
	variables := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(module.cwd),
			"module":  cty.StringVal(filepath.Join(module.cwd, filepath.Dir(filename))),
			"current": cty.StringVal(filepath.Join(module.cwd, filename)),
		}),
	}

	phony := map[string]cty.Value{}
	local := map[string]cty.Value{}
	for _, act := range actions {
		name := act.GetName()
		path := act.Path()
		value := act.CTY()
		switch {
		case path.HasPrefix(dataPrefix):
			phony[name] = value
		case path.HasPrefix(localPrefix):
			local[name] = value
		default:
			// only targets for now !!
			variables[name] = value
		}
	}

	variables[lang.DataLabel] = cty.ObjectVal(phony)
	variables[lang.LocalScope] = cty.ObjectVal(local)
	return &hcl.EvalContext{
		Variables: variables,
		Functions: lang.Functions(),
	}, nil
}
