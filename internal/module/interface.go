package module

import (
	"context"
	"fmt"

	"bake/internal/concurrent"
	"bake/internal/functional"
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

func (module Module) Plan(task lang.RawAddress, addresses []lang.RawAddress) ([]lang.Action, hcl.Diagnostics) {
	allDeps, diags := topo.AllDependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	taskDependencies := allDeps[lang.AddressToString(task)]
	result := concurrent.NewSlice[lang.Action]()
	coordinator := concurrent.NewCoordinator(context.TODO(), concurrent.DefaultParallelism)
	for _, dep := range taskDependencies {
		innerDepedencies := allDeps[lang.AddressToString(dep)]
		depIDs := functional.Map(innerDepedencies[:len(innerDepedencies)-1], lang.RawAddressToString)
		taskID := lang.AddressToString(dep)
		dep := dep // // https://golang.org/doc/faq#closures_and_goroutines

		coordinator.Do(taskID, depIDs, func() error {
			eval := module.Context(dep, result.Items())
			actions, diags := dep.Decode(eval)
			if diags.HasErrors() {
				return diags
			}

			if !dep.GetPath().HasPrefix(lang.DataPrefix) {
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

func (module Module) GetTask(name string, addresses []lang.RawAddress) (lang.RawAddress, hcl.Diagnostics) {
	for _, address := range addresses {
		if lang.PathString(address.GetPath()) != name {
			continue
		}

		return address, nil
	}

	options := functional.Map(addresses, lang.RawAddressToString)
	suggestion := functional.Suggest(name, options)
	summary := "couldn't find any target with name " + name
	if suggestion != "" {
		summary += fmt.Sprintf(`. Did you mean "%s"`, suggestion)
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  summary,
	}}
}
