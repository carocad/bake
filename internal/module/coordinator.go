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
	boundedPool *errgroup.Group
	// use an unbounded go routine pool to wait for the result of the bounded pool
	unboundedPool *errgroup.Group
	waiting       *concurrent.Map[lang.Address, *sync.WaitGroup]
	actions       *concurrent.Slice[lang.Action]
	eval          lang.ContextData
}

func NewCoordinator(ctx context.Context, eval lang.ContextData) Coordinator {
	// todo: do I need to return the context?
	bounded, _ := errgroup.WithContext(ctx)
	bounded.SetLimit(int(eval.Parallelism))
	unbounded, _ := errgroup.WithContext(ctx)
	return Coordinator{
		boundedPool:   bounded,
		unboundedPool: unbounded,
		waiting:       concurrent.NewMapBy[lang.Address, *sync.WaitGroup](lang.AddressToString[lang.Address]),
		actions:       concurrent.NewSlice[lang.Action](),
		eval:          eval,
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

	taskDependencies, _ := allDependencies.Get(task)
	for _, address := range taskDependencies {
		// keep a reference to the original value due to closure and goroutine
		// https://golang.org/doc/faq#closures_and_goroutines
		address := address
		// initialize this dependency wait group so that other goroutines can wait for it
		promise := &sync.WaitGroup{}
		promise.Add(1)
		coordinator.waiting.Put(address, promise)
		// use the unbounded routine pool to schedule all tasks. Most of them will
		// block anyway due to coordinator.waitFor() call inside, however this way
		// the tasks only wait for their dependencies instead of being bound by the
		// order in which they were executed
		coordinator.unboundedPool.Go(func() error {
			defer promise.Done()

			// get the dependencies of this task dependency
			addressDependencies, _ := allDependencies.Get(address)
			// wait for all routines to finish so that we get all actions
			// we need to remove the last element since it is the address itself
			err := coordinator.waitFor(addressDependencies[:len(addressDependencies)-1])
			if err != nil {
				return err
			}

			actions, diags := address.Decode(coordinator.eval.Context(address, coordinator.actions.Items()))
			if diags.HasErrors() {
				return diags
			}

			coordinator.actions.Extend(actions)
			// on dryRun we only apply the data refreshing
			if coordinator.eval.DryRun && !address.GetPath().HasPrefix(lang.DataPrefix) {
				return nil
			}

			promise.Add(len(actions))
			for _, action := range actions {
				action := action
				// use the bounded routine pool to avoid overloading the OS with possibly
				// CPU heavy tasks
				coordinator.boundedPool.Go(func() error {
					defer promise.Done()

					diags := action.Apply()
					if diags.HasErrors() {
						return diags
					}

					return nil
				})
			}
			return nil
		})
	}

	err := coordinator.unboundedPool.Wait()
	if diags, ok := err.(hcl.Diagnostics); ok {
		return nil, diags
	}

	err = coordinator.boundedPool.Wait()
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
