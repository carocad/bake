package module

import (
	"context"

	"bake/internal/concurrent"
	"bake/internal/functional"
	"bake/internal/lang"
	"bake/internal/topo"
	"github.com/hashicorp/hcl/v2"
)

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

		dep := dep // // https://golang.org/doc/faq#closures_and_goroutines
		coordinator.Do(lang.AddressToString(dep), depIDs, func() error {
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
