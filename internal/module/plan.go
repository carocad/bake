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
	dependencies, diags := topo.AllDependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	taskDependencies := dependencies[lang.AddressToString(task)]
	result := concurrent.NewSlice[lang.Action]()
	coordinator := concurrent.NewCoordinator(context.TODO(), concurrent.DefaultParallelism)
	for _, address := range taskDependencies {
		// get the dependencies of this task dependency
		addrDeps := dependencies[lang.AddressToString(address)]
		// we need to remove the last element since it is the address itself
		depIDs := functional.Map(addrDeps[:len(addrDeps)-1], lang.AddressToString[lang.RawAddress])

		// keep a reference to the original value due to closure and goroutine
		// https://golang.org/doc/faq#closures_and_goroutines
		dep := address
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

	// wait for all routines to finish so that we get all actions
	err := coordinator.Wait()
	if diags, ok := err.(hcl.Diagnostics); ok {
		return nil, diags
	}

	if err != nil {
		panic(err)
	}

	return result.Items(), nil
}
