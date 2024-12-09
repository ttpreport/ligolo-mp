package memstore

import "sync"

type Syncmap[K comparable, V any] struct {
	mutex sync.RWMutex
	Data  map[K]V
}

func NewSyncmap[K comparable, V any]() *Syncmap[K, V] {
	return &Syncmap[K, V]{
		Data: make(map[K]V),
	}
}

func (mem *Syncmap[K, V]) Exists(key K) bool {
	defer mem.mutex.RUnlock()
	mem.mutex.RLock()

	_, exists := mem.Data[key]
	return exists
}

func (mem *Syncmap[K, V]) All() map[K]V {
	defer mem.mutex.RUnlock()
	mem.mutex.RLock()

	return mem.Data
}

func (mem *Syncmap[K, V]) Get(key K) V {
	if !mem.Exists(key) {
		return *new(V)
	}

	defer mem.mutex.RUnlock()
	mem.mutex.RLock()

	return mem.Data[key]
}

func (mem *Syncmap[K, V]) Set(key K, value V) {
	defer mem.mutex.Unlock()
	mem.mutex.Lock()

	mem.Data[key] = value
}

func (mem *Syncmap[K, V]) Delete(key K) {
	defer mem.mutex.Unlock()
	mem.mutex.Lock()

	delete(mem.Data, key)
}
