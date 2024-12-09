package session

import (
	"github.com/ttpreport/ligolo-mp/internal/storage"
	"github.com/ttpreport/ligolo-mp/pkg/memstore"
)

type SessionRepository struct {
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
	return ss.storage.GetAll()
}

func (ss *SessionRepository) GetOne(id string) *Session {
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
	if sess.IsConnected {
		ss.connections.Set(sess.ID, sess)
	} else {
		ss.connections.Delete(sess.ID)
	}

	return ss.storage.Set(sess.Hash(), sess)
}

func (ss *SessionRepository) Remove(sess *Session) error {
	ss.connections.Delete(sess.ID)
	return ss.storage.Del(sess.Hash())
}

func (ss *SessionRepository) RemoveAll() error {
	return ss.storage.DelAll()
}
