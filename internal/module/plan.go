package module

import (
	"context"

	"bake/internal/concurrent"
	"bake/internal/lang"
	"bake/internal/topo"

	"github.com/hashicorp/hcl/v2"
)

func (module Module) Do(task lang.RawAddress, addresses []lang.RawAddress, dryRun bool) ([]lang.Action, hcl.Diagnostics) {
	dependencies, diags := topo.AllDependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	taskDependencies, _ := dependencies.Get(task)
	result := concurrent.NewSlice[lang.Action]()
	coordinator := concurrent.NewCoordinator[lang.RawAddress](context.TODO(), concurrent.DefaultParallelism)
	for _, address := range taskDependencies {
		// get the dependencies of this task dependency
		addrDeps, _ := dependencies.Get(address)
		// we need to remove the last element since it is the address itself
		depIDs := addrDeps[:len(addrDeps)-1]
		// keep a reference to the original value due to closure and goroutine
		// https://golang.org/doc/faq#closures_and_goroutines
		dep := address
		coordinator.Do(dep, depIDs, func() error {
			eval := module.Context(dep, result.Items())
			actions, diags := dep.Decode(eval)
			if diags.HasErrors() {
				return diags
			}

			result.Extend(actions)
			// on dryRun we only apply the data refreshing
			if !dryRun && !dep.GetPath().HasPrefix(lang.DataPrefix) {
				return nil
			}

			// TODO: make this run on parallel -> I will need this since for_each
			// implementation will make each address return multiple actions :/
			for _, action := range actions {
				diags := action.Apply()
				if diags.HasErrors() {
					return diags
				}
			}

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
