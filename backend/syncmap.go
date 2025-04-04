package main

import (
	"maps"
	"slices"
	"sync"
)

type syncMap[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

func newSyncMap[K comparable, V any]() *syncMap[K, V] {
	return &syncMap[K, V]{
		mu:   sync.RWMutex{},
		data: make(map[K]V),
	}
}

func (m *syncMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *syncMap[K, V]) Overwrite(v map[K]V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = maps.Clone(v)
}

func (m *syncMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	return v, ok
}

func (m *syncMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

func (m *syncMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.Collect(maps.Keys(m.data))
}

func (m *syncMap[K, V]) RangeFunc(fn func(k K, v V)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.data {
		fn(k, v)
	}
}

func (m *syncMap[K, V]) Data() map[K]V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return maps.Clone(m.data)
}
