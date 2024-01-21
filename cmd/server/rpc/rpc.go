package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"net"

	"github.com/ttpreport/ligolo-mp/assets"
	"github.com/ttpreport/ligolo-mp/internal/common/config"
	"github.com/ttpreport/ligolo-mp/internal/common/events"
	"github.com/ttpreport/ligolo-mp/internal/core/agents"
	"github.com/ttpreport/ligolo-mp/internal/core/certs"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
	"github.com/ttpreport/ligolo-mp/internal/core/tuns"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type ligoloServer struct {
	pb.UnimplementedLigoloServer
	operators    map[string]pb.Ligolo_JoinServer
	tuns         *tuns.Tuns
	agents       *agents.Agents
	ligoloConfig *config.Config
	storage      *storage.Store
}

func (s *ligoloServer) Join(in *pb.Empty, stream pb.Ligolo_JoinServer) error {
	slog.Debug("Received request to join")
	p, ok := peer.FromContext(stream.Context())

	if !ok {
		return errors.New("unknown error reading grpc context")
	}

	tlsInfo := p.AuthInfo.(credentials.TLSInfo)
	operatorName := tlsInfo.State.VerifiedChains[0][0].Subject.CommonName

	if _, ok := s.operators[operatorName]; ok {
		return errors.New("operator already connected")
	}

	events.EventStream <- events.Event{Type: events.OperatorNew, Data: operatorName}

	s.operators[operatorName] = stream

	<-stream.Context().Done()

	delete(s.operators, operatorName)

	events.EventStream <- events.Event{Type: events.OperatorLost, Data: operatorName}

	return nil
}

func (s *ligoloServer) ListAgents(ctx context.Context, in *pb.Empty) (*pb.ListAgentsResp, error) {
	slog.Debug("Received request to list agents", slog.Any("in", in))
	agents := &pb.ListAgentsResp{Agents: []*pb.Agent{}}

	for _, agent := range s.agents.List() {
		agents.Agents = append(agents.Agents, agent.Proto())
	}

	return agents, nil
}

func (s *ligoloServer) RelayStart(ctx context.Context, in *pb.RelayStartReq) (*pb.Empty, error) {
	slog.Debug("Received request to start relay", slog.Any("in", in))
	tun := s.tuns.GetOne(in.GetTunAlias())
	return &pb.Empty{}, s.agents.StartRelay(in.GetAgentAlias(), tun)
}

func (s *ligoloServer) RelayStop(ctx context.Context, in *pb.RelayStopReq) (*pb.Empty, error) {
	slog.Debug("Received request to stop relay", slog.Any("in", in))
	return &pb.Empty{}, s.agents.StopRelay(in.GetAgentAlias())
}

func (s *ligoloServer) ListTuns(ctx context.Context, in *pb.Empty) (*pb.ListTunsResp, error) {
	slog.Debug("Received request to list TUNs", slog.Any("in", in))
	tuns := &pb.ListTunsResp{Tuns: []*pb.Tun{}}

	for _, tun := range s.tuns.List() {
		tuns.Tuns = append(tuns.Tuns, tun.Proto())
	}

	return tuns, nil
}

func (s *ligoloServer) NewTun(ctx context.Context, in *pb.NewTunReq) (*pb.Empty, error) {
	slog.Debug("Received request to create TUN", slog.Any("in", in))
	_, err := s.tuns.Create(in.Name, in.IsLoopback)
	if err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func (s *ligoloServer) DelTun(ctx context.Context, in *pb.DelTunReq) (*pb.Empty, error) {
	slog.Debug("Received request to delete tun", slog.Any("in", in))
	return &pb.Empty{}, s.tuns.Destroy(in.TunAlias)
}

func (s *ligoloServer) NewRoute(ctx context.Context, in *pb.NewRouteReq) (*pb.NewRouteResp, error) {
	slog.Debug("Received request to create route", slog.Any("in", in))
	_, cidr, err := net.ParseCIDR(in.Cidr)
	if err != nil {
		return nil, err
	}

	if overlappingTun := s.tuns.RouteOverlaps(cidr.String()); overlappingTun != nil {
		if !in.Force {
			return &pb.NewRouteResp{OverlappingTun: overlappingTun.Proto()}, nil
		}
	}
	return &pb.NewRouteResp{}, s.tuns.AddRoute(in.TunAlias, cidr.String())
}

func (s *ligoloServer) DelRoute(ctx context.Context, in *pb.DelRouteReq) (*pb.Empty, error) {
	slog.Debug("Received request to delete route", slog.Any("in", in))
	_, cidr, err := net.ParseCIDR(in.Cidr)
	if err != nil {
		return nil, err
	}

	return &pb.Empty{}, s.tuns.DeleteRoute(in.TunAlias, cidr.String())
}

func (s *ligoloServer) NewListener(ctx context.Context, in *pb.NewListenerReq) (*pb.Empty, error) {
	slog.Debug("Received request to create listener", slog.Any("in", in))
	return &pb.Empty{}, s.agents.NewListener(in.AgentAlias, in.Protocol, in.From, in.To)
}

func (s *ligoloServer) DelListener(ctx context.Context, in *pb.DelListenerReq) (*pb.Empty, error) {
	slog.Debug("Received request to delete listener", slog.Any("in", in))
	return &pb.Empty{}, s.agents.DeleteListener(in.AgentAlias, in.ListenerAlias)
}

func (s *ligoloServer) ListListeners(ctx context.Context, in *pb.Empty) (*pb.ListListenersResp, error) {
	slog.Debug("Received request to list listeners", slog.Any("in", in))
	listeners := &pb.ListListenersResp{
		Listeners: []*pb.Listener{},
	}

	for _, agent := range s.agents.List() {
		for _, listener := range agent.Listeners.List() {
			pbListener := listener.Proto()
			pbListener.Agent = agent.Proto()
			listeners.Listeners = append(listeners.Listeners, pbListener)
		}
	}

	return listeners, nil
}

func (s *ligoloServer) GenerateAgent(ctx context.Context, in *pb.GenerateAgentReq) (*pb.GenerateAgentResp, error) {
	dbcacert, err := s.storage.GetCert("_ca_")
	if err != nil {
		return nil, err
	}
	CACert := string(dbcacert.Certificate[:])

	cert, key, err := certs.GenerateCert("", dbcacert.Certificate, dbcacert.Key)
	if err != nil {
		return nil, err
	}
	AgentCert := string(cert[:])
	AgentKey := string(key[:])

	server := in.Server
	if in.Server == "" {
		server = s.ligoloConfig.ListenInterface
	}

	result, err := assets.CompileAgent(in.GOOS, in.GOARCH, in.Obfuscate, in.SocksServer, in.SocksUser, in.SocksPass, server, CACert, AgentCert, AgentKey)
	if err != nil {
		return nil, err
	}

	return &pb.GenerateAgentResp{AgentBinary: result}, nil
}

func (s *ligoloServer) ListCerts(ctx context.Context, in *pb.Empty) (*pb.ListCertsResp, error) {
	slog.Debug("Received request to list certs", slog.Any("in", in))

	dbcerts, err := s.storage.GetCerts()
	if err != nil {
		return nil, err
	}

	certs := &pb.ListCertsResp{Certs: []*pb.Cert{}}

	for _, dbcert := range dbcerts {
		certBlock, _ := pem.Decode(dbcert.Certificate)
		if certBlock == nil {
			continue
		}

		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			continue
		}

		certs.Certs = append(certs.Certs, &pb.Cert{
			Name:       dbcert.Name,
			ExpiryDate: cert.NotAfter.String(),
		})
	}

	return certs, nil
}

func (s *ligoloServer) RegenCerts(ctx context.Context, in *pb.RegenCertsReq) (*pb.Empty, error) {
	slog.Debug("Received request to regenerate certs", slog.Any("in", in))

	err := s.storage.DelCert(in.Name)
	if err != nil {
		return nil, err
	}

	dbCaData, err := s.storage.GetCert("_ca_")
	if err != nil {
		return nil, err
	}

	cert, key, err := certs.GenerateCert(in.Name, dbCaData.Certificate, dbCaData.Key)
	if err != nil {
		return nil, err
	}

	_, err = s.storage.AddCert(in.Name, cert, key)
	if err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func Run(config *config.Config, storage *storage.Store, tunList *tuns.Tuns, agentList *agents.Agents) error {
	lis, err := net.Listen("tcp", config.OperatorAddr)
	if err != nil {
		panic(err)
	}

	dbservercert, err := storage.GetCert("_server_")
	if err != nil {
		panic(err)
	}

	tlsCert, err := tls.X509KeyPair(dbservercert.Certificate, dbservercert.Key)
	if err != nil {
		panic(err)
	}

	dbcacert, err := storage.GetCert("_ca_")
	if err != nil {
		panic(err)
	}
	certpool := x509.NewCertPool()
	if ok := certpool.AppendCertsFromPEM(dbcacert.Certificate); !ok {
		return errors.New("malformed CA certificate")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{tlsCert},
		ClientCAs:          certpool,
		RootCAs:            certpool,
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
	}

	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))

	ligoloServer := &ligoloServer{
		operators:    make(map[string]pb.Ligolo_JoinServer),
		tuns:         tunList,
		agents:       agentList,
		ligoloConfig: config,
		storage:      storage,
	}
	pb.RegisterLigoloServer(grpcServer, ligoloServer)
	slog.Debug("server listening", slog.Any("addr", lis.Addr()))

	go ligoloServer.HandleEvents()

	return grpcServer.Serve(lis)
}
