package concurrent

import "sync"

type Slice[T any] struct {
	mutex sync.RWMutex
	items []T
}

func NewSlice[T any]() *Slice[T] {
	return &Slice[T]{
		items: make([]T, 0),
	}
}

func (slice *Slice[T]) Append(item T) {
	slice.mutex.Lock()
	defer slice.mutex.Unlock()

	slice.items = append(slice.items, item)
}

func (slice *Slice[T]) Extend(items []T) {
	slice.mutex.Lock()
	defer slice.mutex.Unlock()

	slice.items = append(slice.items, items...)
}

func (slice *Slice[T]) Items() []T {
	slice.mutex.RLock()
	defer slice.mutex.RUnlock()

	result := make([]T, len(slice.items))
	copy(result, slice.items)
	return result
}
