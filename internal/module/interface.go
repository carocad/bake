package module

import (
	"context"

	"bake/internal/concurrent"
	"bake/internal/lang"
	"bake/internal/topo"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Module struct {
	// name by which the module is known; by convention the root module
	// doesn't have a name as it is "global"
	name string
	cwd  string
}

func (module Module) Plan(target string, filePartials map[string][]lang.RawAddress) ([]lang.Action, hcl.Diagnostics) {
	for _, addresses := range filePartials {
		for _, act := range addresses {

			if lang.PathString(act.Path()) != target {
				continue
			}

			deps, diags := topo.Dependencies(act, filePartials, lang.GlobalPrefixes)
			if diags.HasErrors() {
				return nil, diags
			}

			result := concurrent.NewSlice[lang.Action]()
			coordinator := concurrent.NewCoordinator(context.TODO(), concurrent.DefaultParallelism)
			for _, dep := range deps {
				requires, diags := topo.Dependencies(dep, filePartials, lang.GlobalPrefixes)
				if diags.HasErrors() {
					return nil, diags
				}

				ids := lang.DependencyIds(requires[:len(requires)-1])
				dep := dep // // https://golang.org/doc/faq#closures_and_goroutines
				coordinator.Do(lang.PathString(dep.Path()), ids, func() error {
					eval := module.Context(dep, filePartials, result.Items())
					actions, diags := dep.Decode(eval)
					if diags.HasErrors() {
						return diags
					}

					if !dep.Path().HasPrefix(lang.DataPrefix) {
						result.Extend(actions)
						return nil
					}

					// we need to refresh before the next actions are loaded since
					// they depend on the data values
					for _, action := range actions {
						diags := action.Apply()
						if diags.HasErrors() {
							return diags
						}
					}

					result.Extend(actions)
					return nil
				})
			}
			err := coordinator.Wait()
			if diags, ok := err.(hcl.Diagnostics); ok {
				return nil, diags
			}

			if err != nil {
				panic(err)
			}

			return result.Items(), nil
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
