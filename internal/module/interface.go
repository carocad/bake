package module

import (
	"bake/internal/lang"
	"bake/internal/topo"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

var (
	dataPrefix  = cty.GetAttrPath(lang.DataLabel)
	localPrefix = cty.GetAttrPath(lang.LocalScope)
	pathPrefix  = cty.GetAttrPath(lang.PathScope)
	// globalPrefixes are those automatically injected by bake instead of defined by
	// user input
	globalPrefixes = cty.NewPathSet(pathPrefix)
)

type Module struct {
	// name by which the module is known; by convention the root module
	// doesn't have a name as it is "global"
	name string
	cwd  string
}

func (module Module) Plan(target string, filePartials map[string][]lang.RawAddress) ([]lang.Action, hcl.Diagnostics) {
	allActions := make([]lang.Action, 0)
	for filename, addresses := range filePartials {
		for _, act := range addresses {

			if act.Path().HasPrefix(localPrefix) {
				continue
			}

			if act.GetName() != target {
				continue
			}

			deps, diags := topo.Dependencies(act, filePartials, globalPrefixes)
			if diags.HasErrors() {
				return nil, diags
			}

			for _, dep := range deps {
				context, diags := module.currentContext(filename, allActions)
				if diags.HasErrors() {
					return nil, diags
				}

				// data blocks perform effects here as
				// locals might depend on data
				addrs, diagnostics := dep.Decode(context)
				if diagnostics.HasErrors() {
					return nil, diagnostics
				}

				allActions = append(allActions, addrs...)
			}

			return allActions, nil
		}
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "couldn't find any target with name " + target,
	}}
}

func NewRootModule(cwd string) *Module {
	return NewModule("", cwd)
}

func NewModule(name, cwd string) *Module {
	return &Module{name: name, cwd: cwd}
}

func (module Module) Path() cty.Path {
	// root module
	if module.name == "" {
		return cty.Path{}
	}

	return cty.GetAttrPath("module").GetAttr(module.name)
}
