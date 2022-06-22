package promise

import (
	"sync"

	"golang.org/x/sync/errgroup"
)

type Promise[T any] struct {
	err   error
	value T
	wg    *sync.WaitGroup
}

func New[T any](effect func() (T, error)) *Promise[T] {
	return NewWithGroup(&errgroup.Group{}, effect)
}

func NewWithGroup[T any](group *errgroup.Group, effect func() (T, error)) *Promise[T] {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	promise := &Promise[T]{wg: wg}
	group.Go(func() error {
		defer promise.wg.Done()

		value, err := effect()
		if err != nil {
			promise.err = err
			return err
		}

		promise.value = value
		return nil
	})

	return promise
}

func (promise *Promise[T]) Wait() (T, error) {
	promise.wg.Wait()
	return promise.value, promise.err
}

// Await for all promises to complete on a separate goroutine, it might block waiting
// for a group goroutine to be available
func Await[T any](group *errgroup.Group, promises ...*Promise[T]) *Promise[[]T] {
	return NewWithGroup(group, func() ([]T, error) {
		return Wait(promises...)
	})
}

// Wait for promises to complete blocking until all are ready
func Wait[T any](promises ...*Promise[T]) ([]T, error) {
	result := make([]T, len(promises))
	for index, p := range promises {
		v, err := p.Wait()
		if err != nil {
			return nil, err
		}

		result[index] = v
	}

	return result, nil
}
