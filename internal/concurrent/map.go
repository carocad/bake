package concurrent

import (
	"fmt"
	"sync"
)

type Indexer[T any] func(T) string

func Identity[T any](v T) T {
	return v
}

type Map[K, V any] struct {
	mutex   sync.RWMutex
	items   map[string]V
	indexer Indexer[K]
}

func NewMap[K fmt.Stringer, V any]() *Map[K, V] {
	return NewMapBy[K, V](K.String)
}

func NewMapBy[K any, V any](indexer Indexer[K]) *Map[K, V] {
	return &Map[K, V]{
		items:   map[string]V{},
		indexer: indexer,
	}
}

func (m *Map[K, V]) Put(key K, value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	index := m.indexer(key)
	m.items[index] = value
}

func (m *Map[K, V]) Merge(items *Map[K, V]) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for k, v := range items.items {
		m.items[k] = v
	}
}

func (m *Map[K, V]) Get(key K) (V, bool) {
	index := m.indexer(key)

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	item, ok := m.items[index]
	return item, ok
}

// Merge m2 on top of m1 in place!
func Merge[K comparable, V any](m1 map[K]V, m2 map[K]V) map[K]V {
	for k, v := range m2 {
		m1[k] = v
	}

	return m1
}
