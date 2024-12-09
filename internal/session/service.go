package session

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/config"
)

type SessionService struct {
	repo   *SessionRepository
	config *config.Config
}

func NewSessionService(config *config.Config, repo *SessionRepository) *SessionService {
	return &SessionService{
		repo:   repo,
		config: config,
	}
}

func (ss *SessionService) NewSession(multiplex *yamux.Session) (*Session, error) {
	session, err := new()
	if err != nil {
		return nil, err
	}

	if err := session.Connect(multiplex); err != nil {
		return nil, err
	}

	savedSession := ss.repo.GetOne(session.ID)
	if savedSession != nil {
		slog.Debug("a saved session found, checking", slog.Any("saved_session", savedSession))

		if savedSession.IsConnected {
			slog.Debug("connection is a duplicate, aborting")
			session.CleanUp()
			return nil, errors.New("connection is a duplicate")
		}
		slog.Debug("connection is unique, restoring session")

		if err := ss.repo.Save(session); err != nil { // effectively invalidates stored session
			return nil, err
		}

		session.Copy(savedSession)

		if savedSession.IsRelaying {
			if err := ss.StartRelay(session.ID); err != nil {
				slog.Error("could not start relay", slog.Any("error", err))
			}
		}

		slog.Debug("session restored")
	}

	return session, ss.repo.Save(session)
}

func (ss *SessionService) GetSession(sessID string) *Session {
	return ss.repo.GetOne(sessID)
}

func (ss *SessionService) SaveSession(sess *Session) error {
	return ss.repo.Save(sess)
}

func (ss *SessionService) DisconnectSession(sessID string) error {
	sess := ss.GetSession(sessID)
	if sess == nil {
		return fmt.Errorf("session '%s' does not exist", sessID)
	}

	if err := sess.Disconnect(); err != nil {
		slog.Warn("session disconnect encountered an error", slog.Any("error", err))
	}

	sess.CleanUp()

	return ss.repo.Save(sess)
}

func (ss *SessionService) KillSession(sessID string) error {
	sess := ss.GetSession(sessID)
	if sess == nil {
		return fmt.Errorf("session '%s' does not exist", sessID)
	}

	if err := sess.Disconnect(); err != nil {
		slog.Warn("session disconnect encountered an error", slog.Any("error", err))
	}

	sess.CleanUp()

	return ss.repo.Remove(sess)
}

func (ss *SessionService) RenameSession(id string, alias string) error {
	session := ss.repo.GetOne(id)
	if session == nil {
		return fmt.Errorf("session '%s' does not exist", id)
	}
	session.Alias = alias
	return ss.repo.Save(session)
}

func (ss *SessionService) NewRoute(sessionID string, cidr string, isLoopback bool) error {
	slog.Debug("adding new route to session")

	session := ss.repo.GetOne(sessionID)
	if session == nil {
		return fmt.Errorf("session '%s' not found", sessionID)
	}
	slog.Debug("found session in storage", slog.Any("session", session))

	err := session.NewRoute(cidr, isLoopback)
	if err != nil {
		return err
	}
	slog.Debug("route added")

	return ss.repo.Save(session)
}

func (ss *SessionService) RemoveRoute(sessionID string, cidr string) error {
	slog.Debug("removing route from session")
	session := ss.repo.GetOne(sessionID)
	if session == nil {
		return fmt.Errorf("session '%s' not found", sessionID)
	}

	if err := session.RemoveRoute(cidr); err != nil {
		return err
	}

	return ss.repo.Save(session)
}

func (ss *SessionService) StartRelay(sessID string) error {
	slog.Debug("activating relay")
	session := ss.repo.GetOne(sessID)
	if session == nil {
		return fmt.Errorf("session '%s' not found", sessID)
	}
	slog.Debug("got session from storage", slog.Any("session", session))

	if err := session.StartRelay(ss.config.MaxConnectionHandler, ss.config.MaxInFlight); err != nil {
		return err
	}

	return ss.repo.Save(session)
}

func (ss *SessionService) StopRelay(sessID string) error {
	session := ss.repo.GetOne(sessID)
	if session == nil {
		return fmt.Errorf("session '%s' not found", sessID)
	}

	if err := session.StopRelay(); err != nil {
		return err
	}

	return ss.repo.Save(session)
}

func (ss *SessionService) NewRedirector(sessID string, proto string, from string, to string) error {
	session := ss.repo.GetOne(sessID)
	if session == nil {
		return fmt.Errorf("session '%s' not found", sessID)
	}

	if err := session.NewRedirector(proto, from, to); err != nil {
		return err
	}

	return ss.repo.Save(session)
}

func (ss *SessionService) RemoveRedirector(sessID string, redirectorID string) error {
	session := ss.repo.GetOne(sessID)
	if session == nil {
		return fmt.Errorf("session '%s' not found", sessID)
	}

	if err := session.RemoveRedirector(redirectorID); err != nil {
		return err
	}

	return ss.repo.Save(session)
}

func (ss *SessionService) GetAll() ([]*Session, error) {
	return ss.repo.GetAll()
}

func (ss *SessionService) RemoveAll() error {
	return ss.repo.RemoveAll()
}

func (ss *SessionService) RouteOverlaps(cidr string) (*Session, string) {
	slog.Debug("looking for route overlaps")
	sessions, err := ss.GetAll()
	if err != nil {
		slog.Debug("could not check overlap", slog.Any("err", err))
		return nil, ""
	}

	for _, sess := range sessions {
		if route, isOverlap := sess.RouteOverlaps(cidr); isOverlap {
			return sess, route
		}
	}

	return nil, ""
}

func (ss *SessionService) CleanUp() error {
	sessions, err := ss.repo.GetAll()
	if err != nil {
		return err
	}

	for _, session := range sessions {
		session.Disconnect()
		session.CleanUp()
		ss.repo.Save(session)
	}

	return nil
}
