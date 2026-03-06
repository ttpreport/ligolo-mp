package session

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/v2/internal/config"
	"github.com/ttpreport/ligolo-mp/v2/internal/netstack/tunlink"
	"github.com/ttpreport/ligolo-mp/v2/internal/route"
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

func (ss *SessionService) KillSession(sessID string) (*Session, error) {
	sess := ss.GetSession(sessID)
	if sess == nil {
		return nil, fmt.Errorf("session '%s' does not exist", sessID)
	}

	sess.Disconnect()
	sess.CleanUp()

	return sess, ss.repo.Remove(sess)
}

func (ss *SessionService) RenameSession(id string, alias string) error {
	session := ss.repo.GetOne(id)
	if session == nil {
		return fmt.Errorf("session '%s' does not exist", id)
	}
	session.Alias = alias
	return ss.repo.Save(session)
}

func (ss *SessionService) NewRoute(sessionID string, cidr string, metric int, isLoopback bool) error {
	slog.Debug("adding new route to session")

	session := ss.repo.GetOne(sessionID)
	if session == nil {
		return fmt.Errorf("session '%s' not found", sessionID)
	}
	slog.Debug("found session in storage", slog.Any("session", session))

	err := session.NewRoute(cidr, metric, isLoopback)
	if err != nil {
		return err
	}
	slog.Debug("route added")

	return ss.repo.Save(session)
}

func (ss *SessionService) RemoveRoute(sessionID string, routeID string) (*route.Route, error) {
	slog.Debug("removing route from session")
	session := ss.repo.GetOne(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session '%s' not found", sessionID)
	}

	route, err := session.RemoveRoute(routeID)
	if err != nil {
		return nil, err
	}

	return route, ss.repo.Save(session)
}

func (ss *SessionService) EditRoute(sessionID string, routeID string, cidr string, metric int, isLoopback bool) (*route.Route, error) {
	session := ss.repo.GetOne(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session '%s' not found", sessionID)
	}

	oldRoute, err := session.RemoveRoute(routeID)
	if err != nil {
		return nil, err
	}

	if err := session.NewRoute(cidr, metric, isLoopback); err != nil {
		// restore kernel state; error here is best-effort
		session.NewRoute(oldRoute.Cidr.String(), oldRoute.Metric, oldRoute.IsLoopback) //nolint:errcheck
		return nil, err
	}

	return oldRoute, ss.repo.Save(session)
}

func (ss *SessionService) MoveRoute(fromSessID string, routeID string, toSessID string) (*route.Route, error) {
	fromSess := ss.repo.GetOne(fromSessID)
	if fromSess == nil {
		return nil, fmt.Errorf("session '%s' not found", fromSessID)
	}

	toSess := ss.repo.GetOne(toSessID)
	if toSess == nil {
		return nil, fmt.Errorf("session '%s' not found", toSessID)
	}

	oldRoute, err := fromSess.RemoveRoute(routeID)
	if err != nil {
		return nil, err
	}

	if err := toSess.NewRoute(oldRoute.Cidr.String(), oldRoute.Metric, oldRoute.IsLoopback); err != nil {
		// restore kernel state; error here is best-effort
		fromSess.NewRoute(oldRoute.Cidr.String(), oldRoute.Metric, oldRoute.IsLoopback) //nolint:errcheck
		return nil, err
	}

	if err := ss.repo.Save(fromSess); err != nil {
		return nil, err
	}

	return oldRoute, ss.repo.Save(toSess)
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

func (ss *SessionService) UpdateLastSeen(sessID string) error {
	slog.Debug("updating last seen")
	session := ss.repo.GetOne(sessID)
	if session == nil {
		return fmt.Errorf("session '%s' not found", sessID)
	}
	slog.Debug("got session from storage", slog.Any("session", session))

	session.LastSeen = time.Now()

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

func (ss *SessionService) Traceroute(address string) ([]route.Trace, error) {
	slog.Debug("tracing address")
	ip := net.ParseIP(address)
	if ip == nil {
		return nil, fmt.Errorf("malformed IP address")
	}

	routes, err := tunlink.GetRoute(ip)
	if err != nil {
		return nil, err
	}

	sessions, err := ss.GetAll()
	if err != nil {
		return nil, err
	}

	sessionTuns := make(map[string]*Session)
	for _, session := range sessions {
		sessionTuns[session.Tun.Name] = session
	}

	var result []route.Trace
	for _, linkroute := range routes {
		iface, err := tunlink.GetName(linkroute.LinkIndex)
		if err != nil {
			return nil, err
		}

		if routingSession, ok := sessionTuns[iface]; ok {
			result = append(result, route.Trace{
				IsInternal: true,
				Session:    routingSession.GetName(),
				Iface:      iface,
				Metric:     uint(linkroute.Priority),
			})
		} else {
			var via string
			if linkroute.Via != nil {
				via = linkroute.Via.String()
			}

			result = append(result, route.Trace{
				IsInternal: false,
				Iface:      iface,
				Via:        via,
				Metric:     uint(linkroute.Priority),
			})
		}
	}

	return result, nil
}

func (ss *SessionService) Init() error {
	return nil
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
