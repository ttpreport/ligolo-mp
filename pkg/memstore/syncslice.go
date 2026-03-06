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
	mem.mutex.RLock()
	defer mem.mutex.RUnlock()

	return len(mem.Data) > key
}

func (mem *Syncslice[V]) All() []V {
	mem.mutex.RLock()
	defer mem.mutex.RUnlock()

	cp := make([]V, len(mem.Data))
	copy(cp, mem.Data)
	return cp
}

func (mem *Syncslice[V]) Get(key int) V {
	if !mem.Exists(key) {
		return *new(V)
	}

	mem.mutex.RLock()
	defer mem.mutex.RUnlock()

	return mem.Data[key]
}

func (mem *Syncslice[V]) Append(value V) {
	mem.mutex.Lock()
	defer mem.mutex.Unlock()

	mem.Data = append(mem.Data, value)
}

func (mem *Syncslice[V]) Delete(key int) {
	mem.mutex.Lock()
	defer mem.mutex.Unlock()

	mem.Data[key] = mem.Data[len(mem.Data)-1]
	mem.Data = mem.Data[:len(mem.Data)-1]
}
