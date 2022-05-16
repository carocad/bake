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

func (module Module) parentContext(addr lang.RawAddress, filePartials map[string][]lang.RawAddress) (*hcl.EvalContext, hcl.Diagnostics) {
	addrFile := ""
	for filename, addresses := range filePartials {
		for _, address := range addresses {
			if address.Path().Equals(addr.Path()) {
				addrFile = filename
				break
			}
		}
	}

	if addrFile == "" {
		panic("couldn't find address on read files, please notify bake developers")
	}

	variables := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(module.cwd),
			"module":  cty.StringVal(filepath.Join(module.cwd, filepath.Dir(addrFile))),
			"current": cty.StringVal(filepath.Join(module.cwd, addrFile)),
		}),
	}

	return &hcl.EvalContext{
		Variables: variables,
		Functions: lang.Functions(),
	}, nil
}

func (module Module) childContext(child *hcl.EvalContext, actions []lang.Action) (*hcl.EvalContext, hcl.Diagnostics) {
	child.Variables = map[string]cty.Value{}
	data := map[string]cty.Value{}
	local := map[string]cty.Value{}
	for _, act := range actions {
		name := act.GetName()
		path := act.Path()
		value := act.CTY()
		switch {
		case path.HasPrefix(lang.DataPrefix):
			data[name] = value
		case path.HasPrefix(lang.LocalPrefix):
			local[name] = value
		default:
			// only targets for now !!
			child.Variables[name] = value
		}
	}

	child.Variables[lang.DataLabel] = cty.ObjectVal(data)
	child.Variables[lang.LocalScope] = cty.ObjectVal(local)
	return child, nil
}
