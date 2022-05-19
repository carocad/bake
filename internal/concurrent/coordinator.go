package concurrent

import (
	"bake/internal/lang"
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

const DefaultParallelism = 4

// Coordinator executes tasks in parallel respecting the dependencies between each task
type Coordinator[T lang.Address] struct {
	pool    *errgroup.Group
	waiting *Map[T, *sync.WaitGroup]
}

func NewCoordinator[T lang.Address](ctx context.Context, parallelism int) Coordinator[T] {
	group, _ := errgroup.WithContext(ctx) // todo: do I need to return the context?
	group.SetLimit(parallelism)
	return Coordinator[T]{
		pool:    group,
		waiting: NewMapBy[T, *sync.WaitGroup](lang.AddressToString[T]),
	}
}

// Do task with id on a separate go routine after its dependencies are done. All dependencies MUST
// have a previously registered task, otherwise the entire task coordinator
// is stopped and an error is returned
func (coordinator *Coordinator[T]) Do(id T, dependencies []T, f func() error) {
	// have to execute this otherwise there would be a race condition between the routines
	promise := &sync.WaitGroup{}
	promise.Add(1)
	coordinator.waiting.Put(id, promise)

	var err error
	for _, dep := range dependencies {
		waiter, ok := coordinator.waiting.Get(dep)
		if !ok {
			err = fmt.Errorf("missing task %s while processing %s",
				lang.AddressToString(dep),
				lang.AddressToString(id))
			break
		}

		waiter.Wait()
	}

	coordinator.pool.Go(func() error {
		defer promise.Done()
		if err != nil {
			return err
		}

		return f()
	})
}

// Wait for all task to complete. If any task returned an error
// it will be returned here
func (coordinator *Coordinator[T]) Wait() error {
	return coordinator.pool.Wait()
}
