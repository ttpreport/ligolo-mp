package session

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sort"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/v2/internal/protocol"
	"github.com/ttpreport/ligolo-mp/v2/internal/route"
	"github.com/ttpreport/ligolo-mp/v2/internal/tun"
	"github.com/ttpreport/ligolo-mp/v2/pkg/memstore"
	pb "github.com/ttpreport/ligolo-mp/v2/protobuf"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Session struct {
	ID          string
	Alias       string
	IsConnected bool
	IsRelaying  bool
	Redirectors *memstore.Syncmap[string, Redirector]
	Tun         *tun.Tun
	Hostname    string
	Interfaces  *memstore.Syncslice[protocol.NetInterface]
	Multiplex   *yamux.Session `json:"-"`
	FirstSeen   time.Time
	LastSeen    time.Time
}

type Redirector struct {
	ID       string
	Protocol string
	From     string
	To       string
}

func (r *Redirector) Hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(r.Protocol))
	hasher.Write([]byte(r.From))
	hasher.Write([]byte(r.To))
	return hex.EncodeToString(hasher.Sum(nil))
}

func new() (*Session, error) {
	result := &Session{
		Redirectors: memstore.NewSyncmap[string, Redirector](),
		Interfaces:  memstore.NewSyncslice[protocol.NetInterface](),
	}

	tun, err := tun.NewTun()
	if err != nil {
		slog.Error("could not create new tun, aborting")
		return nil, err
	}
	result.Tun = tun
	slog.Debug("new tun created", slog.Any("tun", tun))

	return result, nil
}

func (sess *Session) GetName() string {
	if sess.Alias != "" {
		return sess.Alias
	}

	return sess.Hostname
}

func (sess *Session) GetFirstSeen() time.Time {
	return sess.FirstSeen
}

func (sess *Session) GetLastSeen() time.Time {
	return sess.LastSeen
}

func (sess *Session) RouteOverlaps(cidr string) (string, bool) {
	for _, route := range sess.Tun.GetRoutes() {
		slog.Debug("checking route", slog.Any("route", route))
		RoutePrefix, err := netip.ParsePrefix(route.Cidr.String())
		if err != nil {
			slog.Debug("could not check overlap", slog.Any("err", err))
			return "", false
		}

		cidrToTest, err := netip.ParsePrefix(cidr)
		if err != nil {
			slog.Debug("could not check overlap", slog.Any("err", err))
			return "", false
		}

		if RoutePrefix.Overlaps(cidrToTest) {
			slog.Debug("overlap found", slog.Any("prefix", RoutePrefix))
			return RoutePrefix.String(), true
		}
		slog.Debug("no overlap found")
	}

	return "", false
}

func (sess *Session) NewRoute(cidr string, metric int, isLoopback bool) error {
	if err := sess.Tun.NewRoute(cidr, metric, isLoopback); err != nil {
		return err
	}

	if sess.IsRelaying {
		return sess.Tun.ApplyRoutes()
	}

	return nil
}

func (sess *Session) RemoveRoute(id string) (*route.Route, error) {
	route, err := sess.Tun.RemoveRoute(id)
	if err != nil {
		return nil, err
	}

	if sess.IsRelaying {
		if err := sess.Tun.ApplyRoutes(); err != nil {
			return nil, err
		}
	}

	return route, nil
}

func (sess *Session) Copy(source *Session) {
	sess.Alias = source.Alias
	sess.FirstSeen = source.FirstSeen

	for _, route := range source.Tun.GetRoutes() {
		if err := sess.NewRoute(route.Cidr.String(), route.Metric, route.IsLoopback); err != nil {
			slog.Error("could not create new route", slog.Any("route", route))
			continue
		}
		slog.Debug("route restored", slog.Any("route", route))
	}

	for _, redirector := range source.Redirectors.All() {
		if err := sess.NewRedirector(redirector.Protocol, redirector.From, redirector.To); err != nil {
			slog.Error("could not create new redirector", slog.Any("redirector", redirector))
		}
		slog.Debug("redirector restored", slog.Any("redirector", redirector))
	}
}

func (sess *Session) Connect(multiplex *yamux.Session) error {
	sess.Multiplex = multiplex

	info, err := sess.remoteGetInfo()
	if err != nil {
		return err
	}
	slog.Debug("received network info from remote")

	for _, iface := range info.Interfaces {
		sess.Interfaces.Append(iface)
	}

	sess.Hostname = info.Hostname
	sess.ID = sess.Hash()
	sess.IsConnected = true

	if sess.FirstSeen.IsZero() {
		sess.FirstSeen = time.Now()
	}

	if sess.LastSeen.IsZero() {
		sess.LastSeen = time.Now()
	}

	// Close agent-side redirectors the server no longer tracks.
	// Must run after IsConnected=true so remoteRemoveRedirector actually fires.
	for _, remoteRedirector := range info.Redirectors {
		if !sess.Redirectors.Exists(remoteRedirector.ID) {
			if err := sess.remoteRemoveRedirector(remoteRedirector.ID); err != nil {
				slog.Error("could not remove stale redirector", slog.Any("error", err))
			}
		}
	}
	// Re-open server-tracked redirectors on the agent (used when Connect is
	// called on a session that already has redirectors).
	for _, redirector := range sess.Redirectors.All() {
		if err := sess.NewRedirector(redirector.Protocol, redirector.From, redirector.To); err != nil {
			slog.Error("could not create redirector", slog.Any("error", err))
		}
	}

	return nil
}

func (sess *Session) Disconnect() error {
	sess.IsConnected = false
	return sess.remoteDestroySession()
}

func (sess *Session) StartRelay(maxConnections int, maxInFlight int) error {
	if sess.IsRelaying {
		return fmt.Errorf("relay is already running")
	}

	if sess.IsConnected {
		if err := sess.Tun.Start(sess.Multiplex, maxConnections, maxInFlight); err != nil {
			return err
		}
	}

	sess.IsRelaying = true

	return nil
}

func (sess *Session) StopRelay() error {
	if !sess.IsRelaying {
		return fmt.Errorf("relay is not active")
	}

	sess.Tun.Stop()
	sess.IsRelaying = false

	return nil
}

func (sess *Session) NewRedirector(proto string, from string, to string) error {
	redirector := Redirector{
		Protocol: proto,
		From:     from,
		To:       to,
	}
	redirector.ID = redirector.Hash()

	if sess.IsConnected {
		if err := sess.remoteCreateRedirector(redirector.ID, proto, from, to); err != nil {
			return err
		}
	}

	sess.Redirectors.Set(redirector.ID, redirector)
	return nil
}

func (sess *Session) GetRedirector(redirectorID string) Redirector {
	return sess.Redirectors.Get(redirectorID)
}

func (sess *Session) RemoveRedirector(redirectorID string) error {
	sess.Redirectors.Delete(redirectorID)

	if sess.IsConnected {
		if err := sess.remoteRemoveRedirector(redirectorID); err != nil {
			return err
		}
	}

	return nil
}

func (sess *Session) CleanUp() {
	sess.Tun.Stop()
}

func (sess *Session) IsMultiplexOpen() bool {
	if sess.Multiplex == nil || sess.Multiplex.IsClosed() {
		return false
	}

	return true
}

func (sess *Session) remoteGetInfo() (protocol.InfoReplyPacket, error) {
	if !sess.IsMultiplexOpen() {
		return protocol.InfoReplyPacket{}, fmt.Errorf("multiplex is disconnected")
	}

	stream, err := sess.Multiplex.Open()
	if err != nil {
		return protocol.InfoReplyPacket{}, err
	}
	defer stream.Close()

	protocolEncoder := protocol.NewEncoder(stream)
	protocolDecoder := protocol.NewDecoder(stream)

	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageInfoRequest,
		Payload: protocol.InfoRequestPacket{},
	}); err != nil {
		return protocol.InfoReplyPacket{}, err
	}

	if err := protocolDecoder.Decode(); err != nil {
		return protocol.InfoReplyPacket{}, err
	}

	stream.Close()

	response := protocolDecoder.Envelope.Payload.(protocol.InfoReplyPacket)
	return response, nil
}

func (sess *Session) remoteDestroySession() error {
	if !sess.IsMultiplexOpen() {
		return nil
	}

	stream, err := sess.Multiplex.Open()
	if err != nil {
		return nil
	}
	defer stream.Close()

	protocolEncoder := protocol.NewEncoder(stream)
	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageDisconnectRequest,
		Payload: protocol.DisconnectRequestPacket{},
	}); err != nil {
		return err
	}

	return nil
}

func (sess *Session) remoteCreateRedirector(id string, proto string, from string, to string) error {
	if !sess.IsMultiplexOpen() {
		return fmt.Errorf("multiplex is disconnected")
	}

	stream, err := sess.Multiplex.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	protocolEncoder := protocol.NewEncoder(stream)
	protocolDecoder := protocol.NewDecoder(stream)

	redirectorPacket := protocol.RedirectorRequestPacket{
		ID:      id,
		Network: proto,
		From:    from,
		To:      to,
	}
	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageRedirectorRequest,
		Payload: redirectorPacket,
	}); err != nil {
		return err
	}

	if err := protocolDecoder.Decode(); err != nil {
		return err
	}
	redirectorResponse := protocolDecoder.Envelope.Payload.(protocol.RedirectorResponsePacket)
	if redirectorResponse.Err {
		return errors.New(redirectorResponse.ErrString)
	}

	stream.Close()

	return nil
}

func (sess *Session) remoteRemoveRedirector(id string) error {
	if !sess.IsMultiplexOpen() {
		return fmt.Errorf("multiplex is disconnected")
	}

	stream, err := sess.Multiplex.Open()
	if err != nil {
		return err
	}
	defer stream.Close()

	protocolEncoder := protocol.NewEncoder(stream)
	protocolDecoder := protocol.NewDecoder(stream)

	closeRequest := protocol.RedirectorCloseRequestPacket{ID: id}
	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageRedirectorCloseRequest,
		Payload: closeRequest,
	}); err != nil {
		return err
	}

	if err := protocolDecoder.Decode(); err != nil {
		return err

	}
	response := protocolDecoder.Envelope.Payload

	if err := response.(protocol.RedirectorCloseResponsePacket).Err; err {
		return errors.New(response.(protocol.RedirectorCloseResponsePacket).ErrString)
	}

	stream.Close()

	return nil
}

func (sess *Session) Hash() string {
	hasher := sha1.New()

	ifaces := sess.Interfaces.All()
	sort.SliceStable(ifaces, func(i, j int) bool {
		return ifaces[i].HardwareAddr.String() > ifaces[j].HardwareAddr.String()
	})
	for _, ifaceInfo := range ifaces {
		hasher.Write([]byte(ifaceInfo.HardwareAddr))
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

func (sess *Session) String() string {
	return fmt.Sprintf("ID=%s Alias=%s Hostname=%s", sess.ID, sess.Alias, sess.Hostname)
}

func (sess *Session) Proto() *pb.Session {
	var ifaces []*pb.Interface
	for _, ifaceInfo := range sess.Interfaces.All() {
		ifaces = append(ifaces, &pb.Interface{
			Name: ifaceInfo.Name,
			IPs:  ifaceInfo.Addresses,
		})
	}

	var redirectors []*pb.Redirector
	for _, r := range sess.Redirectors.All() {
		redirectors = append(redirectors, &pb.Redirector{
			ID:       r.ID,
			Protocol: r.Protocol,
			From:     r.From,
			To:       r.To,
		})
	}

	return &pb.Session{
		ID:          sess.ID,
		Alias:       sess.Alias,
		IsConnected: sess.IsConnected,
		IsRelaying:  sess.IsRelaying,
		Redirectors: redirectors,
		Tun:         sess.Tun.Proto(),
		Hostname:    sess.Hostname,
		Interfaces:  ifaces,
		FirstSeen:   timestamppb.New(sess.GetFirstSeen()),
		LastSeen:    timestamppb.New(sess.GetLastSeen()),
	}
}

func ProtoToSession(p *pb.Session) *Session {
	redirectors := memstore.NewSyncmap[string, Redirector]()
	for _, r := range p.Redirectors {
		redirector := Redirector{
			ID:       r.ID,
			Protocol: r.Protocol,
			From:     r.From,
			To:       r.To,
		}

		redirectors.Set(redirector.ID, redirector)
	}

	ifaces := memstore.NewSyncslice[protocol.NetInterface]()
	for _, iface := range p.Interfaces {
		ifaces.Append(protocol.NetInterface{
			Name:      iface.Name,
			Addresses: iface.IPs,
		})
	}

	return &Session{
		ID:          p.ID,
		Alias:       p.Alias,
		IsConnected: p.IsConnected,
		IsRelaying:  p.IsRelaying,
		Redirectors: redirectors,
		Tun:         tun.ProtoToTun(p.Tun),
		Hostname:    p.Hostname,
		Interfaces:  ifaces,
		FirstSeen:   p.FirstSeen.AsTime(),
		LastSeen:    p.LastSeen.AsTime(),
	}
}
