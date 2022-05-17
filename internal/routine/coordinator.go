package routine

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

const DefaultParallelism = 4

// Coordinator executes tasks in parallel respecting the dependencies between each task
type Coordinator struct {
	pool    *errgroup.Group
	ctx     context.Context
	mutex   sync.Mutex                 // protects waiting
	waiting map[string]*sync.WaitGroup // lazily initialized
}

func WithContext(ctx context.Context) Coordinator {
	group, ctx2 := errgroup.WithContext(ctx)
	group.SetLimit(DefaultParallelism)
	return Coordinator{
		pool:    group,
		ctx:     ctx2,
		waiting: map[string]*sync.WaitGroup{},
	}
}

// Do task with id on a separate go routine after its dependencies are done. All dependencies MUST
// have a previously registered task, otherwise the entire task coordinator
// is stopped and an error is returned
func (coordinator *Coordinator) Do(id string, dependencies []string, f func() error) {
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()

	// have to execute this otherwise there would be a race condition between the routines
	promise := &sync.WaitGroup{}
	promise.Add(1)
	coordinator.waiting[id] = promise

	var err error
	for _, dep := range dependencies {
		waiter, ok := coordinator.waiting[dep]
		if !ok {
			err = fmt.Errorf("missing task %s while processing %s", dep, id)
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
func (coordinator *Coordinator) Wait() error {
	return coordinator.pool.Wait()
}
