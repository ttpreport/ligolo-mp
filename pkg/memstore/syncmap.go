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
	mem.mutex.RLock()
	defer mem.mutex.RUnlock()

	_, exists := mem.Data[key]
	return exists
}

func (mem *Syncmap[K, V]) All() map[K]V {
	mem.mutex.RLock()
	defer mem.mutex.RUnlock()

	cp := make(map[K]V, len(mem.Data))
	for k, v := range mem.Data {
		cp[k] = v
	}
	return cp
}

func (mem *Syncmap[K, V]) Get(key K) V {
	mem.mutex.RLock()
	defer mem.mutex.RUnlock()

	return mem.Data[key]
}

func (mem *Syncmap[K, V]) Set(key K, value V) {
	mem.mutex.Lock()
	defer mem.mutex.Unlock()

	mem.Data[key] = value
}

func (mem *Syncmap[K, V]) Delete(key K) {
	mem.mutex.Lock()
	defer mem.mutex.Unlock()

	delete(mem.Data, key)
}
