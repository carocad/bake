package concurrent

import (
	"fmt"
	"sync"
)

type Set[T any] struct {
	mutex   sync.RWMutex
	items   map[string]T
	indexer func(T) string
}

func NewSet[T fmt.Stringer]() *Set[T] {
	return NewSetBy(T.String)
}

func NewSetBy[T any](indexer func(T) string) *Set[T] {
	return &Set[T]{
		items:   map[string]T{},
		indexer: indexer,
	}
}

func (slice *Set[T]) Add(item T) {
	slice.mutex.Lock()
	defer slice.mutex.Unlock()

	index := slice.indexer(item)
	slice.items[index] = item
}

func (slice *Set[T]) Extend(items []T) {
	slice.mutex.Lock()
	defer slice.mutex.Unlock()

	for _, v := range items {
		index := slice.indexer(v)
		slice.items[index] = v
	}
}

func (slice *Set[T]) Get(search T) (T, bool) {
	index := slice.indexer(search)
	slice.mutex.RLock()
	defer slice.mutex.RUnlock()

	item, ok := slice.items[index]
	return item, ok
}
