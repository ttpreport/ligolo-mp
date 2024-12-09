package memstore

import (
	"sync"
)

type Syncslice[V any] struct {
	mutex sync.RWMutex
	Data  []V
}

func NewSyncslice[V any]() *Syncslice[V] {
	return &Syncslice[V]{}
}

func (mem *Syncslice[V]) Exists(key int) bool {
	defer mem.mutex.RUnlock()
	mem.mutex.RLock()

	return len(mem.Data) > key
}

func (mem *Syncslice[V]) All() []V {
	defer mem.mutex.RUnlock()
	mem.mutex.RLock()

	return mem.Data
}

func (mem *Syncslice[V]) Get(key int) V {
	if !mem.Exists(key) {
		return *new(V)
	}

	defer mem.mutex.RUnlock()
	mem.mutex.RLock()

	return mem.Data[key]
}

func (mem *Syncslice[V]) Append(value V) {
	defer mem.mutex.Unlock()
	mem.mutex.Lock()

	mem.Data = append(mem.Data, value)
}

func (mem *Syncslice[V]) Delete(key int) {
	defer mem.mutex.Unlock()
	mem.mutex.Lock()

	mem.Data[key] = mem.Data[len(mem.Data)-1]
	mem.Data = mem.Data[:len(mem.Data)-1]
}
