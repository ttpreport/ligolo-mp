package listeners

import (
	"log/slog"
	"sync"

	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/common/events"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
)

type Listeners struct {
	active map[string]*Listener
	mutex  *sync.RWMutex
	store  *storage.Store
}

func New(storage *storage.Store) *Listeners {
	return &Listeners{
		active: make(map[string]*Listener),
		mutex:  &sync.RWMutex{},
		store:  storage,
	}
}

func (listeners *Listeners) Create(network string, listener_addr string, redirect_addr string, session *yamux.Session) (Listener, error) {
	listeners.mutex.Lock()
	defer listeners.mutex.Unlock()

	var err error

	var alias string
	for i := 0; i < 6; i++ {
		alias = namesgenerator.GetRandomName(i)

		if _, ok := listeners.active[alias]; !ok {
			break
		}
	}

	new_listener, err := newListener(alias, network, listener_addr, redirect_addr, session)

	if err != nil {
		slog.Error("Couldn't create listener",
			slog.Any("reason", err),
		)
		return Listener{}, err
	}

	listeners.active[alias] = &new_listener

	return new_listener, nil
}

func (listeners *Listeners) Restore(listenerId int32, network string, listener_addr string, redirect_addr string, session *yamux.Session) (Listener, error) {
	listeners.mutex.Lock()
	defer listeners.mutex.Unlock()

	var err error

	var alias string
	for i := 0; i < 6; i++ {
		alias = namesgenerator.GetRandomName(i)

		if _, ok := listeners.active[alias]; !ok {
			break
		}
	}

	new_listener, err := restoreListener(alias, listenerId, network, listener_addr, redirect_addr, session)

	if err != nil {
		slog.Error("Couldn't create listener",
			slog.Any("reason", err),
		)
		return Listener{}, err
	}

	listeners.active[alias] = &new_listener

	return new_listener, nil
}

func (listeners *Listeners) Destroy(alias string) error {
	listeners.mutex.Lock()
	defer listeners.mutex.Unlock()

	if listener, ok := listeners.active[alias]; ok {
		if err := listener.Destroy(); err != nil {
			events.EventStream <- events.Event{Type: events.ListenerErr, Data: *listener, Error: err}
			return err
		}

		delete(listeners.active, alias)
	}

	return nil
}

// I know
func (listeners *Listeners) List() map[string]*Listener {
	return listeners.active
}

func (listener *Listeners) Len() int {
	listener.mutex.RLock()
	defer listener.mutex.RUnlock()

	return len(listener.active)
}
