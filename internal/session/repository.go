package session

import (
	"sync"

	"github.com/ttpreport/ligolo-mp/v2/internal/storage"
	"github.com/ttpreport/ligolo-mp/v2/pkg/memstore"
)

// SessionRepository holds two synchronisation domains: the in-memory
// connections map (live sessions) and SQLite (persistence). mu spans both so
// that Save/Remove/GetOne/GetAll always see a consistent view of both stores.
type SessionRepository struct {
	mu          sync.Mutex
	storage     *storage.StoreInstance[Session]
	connections *memstore.Syncmap[string, *Session]
}

var table = "sessions"

func NewSessionRepository(store *storage.Store) (*SessionRepository, error) {
	storeInstance, err := storage.GetInstance[Session](store, table)
	if err != nil {
		return nil, err
	}

	return &SessionRepository{
		storage:     storeInstance,
		connections: memstore.NewSyncmap[string, *Session](),
	}, nil
}

func (ss *SessionRepository) GetAll() ([]*Session, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	sessions, err := ss.storage.GetAll()
	if err != nil {
		return nil, err
	}

	for i, sess := range sessions {
		if live := ss.connections.Get(sess.ID); live != nil {
			sessions[i] = live
		}
	}

	return sessions, nil
}

func (ss *SessionRepository) GetOne(id string) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	connection := ss.connections.Get(id)
	if connection != nil {
		return connection
	}

	result, err := ss.storage.Get(id)
	if err != nil {
		return nil
	}

	return result
}

func (ss *SessionRepository) Save(sess *Session) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if sess.IsConnected {
		ss.connections.Set(sess.ID, sess)
	} else {
		ss.connections.Delete(sess.ID)
	}

	return ss.storage.Set(sess.Hash(), sess)
}

func (ss *SessionRepository) Remove(sess *Session) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.connections.Delete(sess.ID)
	return ss.storage.Del(sess.Hash())
}

func (ss *SessionRepository) RemoveAll() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.connections = memstore.NewSyncmap[string, *Session]()
	return ss.storage.DelAll()
}
