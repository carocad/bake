package concurrent

import (
	"sync"

	"golang.org/x/sync/errgroup"
)

type Promise[T any] struct {
	IsValid bool
	Error   error
	Value   T
	wg      *sync.WaitGroup
}

func NewPromise[T any](effect func() (T, error)) *Promise[T] {
	return NewPromiseGroup(&errgroup.Group{}, effect)
}

func NewPromiseGroup[T any](group *errgroup.Group, effect func() (T, error)) *Promise[T] {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	promise := &Promise[T]{wg: wg}
	group.Go(func() error {
		defer promise.wg.Done()

		value, err := effect()
		promise.IsValid = true
		if err != nil {
			promise.Error = err
			return err
		}

		promise.Value = value
		return nil
	})

	return promise
}

func (promise *Promise[T]) Wait() (T, error) {
	promise.wg.Wait()
	return promise.Value, promise.Error
}

func WaitFor[T any](promises ...*Promise[T]) ([]T, error) {
	return WaitForGroup(&errgroup.Group{}, promises...)
}

func WaitForGroup[T any](group *errgroup.Group, promises ...*Promise[T]) ([]T, error) {
	result := make([]T, len(promises))
	for _, p := range promises {
		v, err := p.Wait()
		if err != nil {
			return nil, err
		}

		result = append(result, v)
	}

	return result, nil
}
