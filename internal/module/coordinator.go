package module

import (
	"bake/internal/concurrent"
	"bake/internal/lang"
	"bake/internal/topo"
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"golang.org/x/sync/errgroup"
)

// Coordinator executes tasks in parallel respecting the dependencies between each task
type Coordinator struct {
	// use a bounded go routine pool for the execution of task commands
	pool    *errgroup.Group
	waiting *concurrent.Map[lang.Address, *sync.WaitGroup]
	actions *concurrent.Slice[lang.Action]
	eval    lang.ContextData
}

func NewCoordinator(ctx context.Context, eval lang.ContextData) Coordinator {
	// todo: do I need to return the context?
	bounded, _ := errgroup.WithContext(ctx)
	bounded.SetLimit(int(eval.Parallelism))
	return Coordinator{
		pool:    bounded,
		waiting: concurrent.NewMapBy[lang.Address, *sync.WaitGroup](lang.AddressToString[lang.Address]),
		actions: concurrent.NewSlice[lang.Action](),
		eval:    eval,
	}
}

// Do task with id on a separate go routine after its dependencies are done. All dependencies MUST
// have a previously registered task, otherwise the entire task coordinator
// is stopped and an error is returned
func (coordinator *Coordinator) Do(task lang.RawAddress, addresses []lang.RawAddress) ([]lang.Action, hcl.Diagnostics) {
	allDependencies, diags := topo.AllDependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	filecache := FileCache{}
	taskDependencies, _ := allDependencies.Get(task)
	for _, address := range taskDependencies {
		// initialize this dependency wait group so that other goroutines can wait for it
		promise := &sync.WaitGroup{}
		coordinator.waiting.Put(address, promise)

		// get the dependencies of this task dependency
		addressDependencies, _ := allDependencies.Get(address)
		// wait for all routines to finish so that we get all actions
		// we need to remove the last element since it is the address itself
		diags := coordinator.waitFor(addressDependencies[:len(addressDependencies)-1])
		if diags.HasErrors() {
			return nil, diags
		}

		evalContext := coordinator.eval.Context(address, coordinator.actions.Items())
		actions, diags := address.Decode(evalContext)
		if diags.HasErrors() {
			return nil, diags
		}

		coordinator.actions.Extend(actions)
		// on dryRun we only apply the data refreshing
		if coordinator.eval.DryRun && !address.GetPath().HasPrefix(lang.DataPrefix) {
			return nil, nil
		}

		promise.Add(len(actions))
		for _, action := range actions {
			// todo: check if action should run here

			// keep a reference to the original value due to closure and goroutine
			// https://golang.org/doc/faq#closures_and_goroutines
			action := action
			// use the bounded routine pool to avoid overloading the OS with possibly
			// CPU heavy tasks
			coordinator.pool.Go(func() error {
				defer promise.Done()

				diags := action.Apply()
				if diags.HasErrors() {
					return diags
				}

				return nil
			})
		}
	}

	err := coordinator.pool.Wait()
	if diags, ok := err.(hcl.Diagnostics); ok {
		return nil, diags
	}

	return coordinator.actions.Items(), nil
}

func (coordinator *Coordinator) waitFor(dependencies []lang.RawAddress) hcl.Diagnostics {
	for _, dep := range dependencies {
		waiter, ok := coordinator.waiting.Get(dep)
		if !ok {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary: fmt.Sprintf("missing task %s",
					lang.AddressToString(dep)),
			}}
		}

		waiter.Wait()
	}

	return nil
}
