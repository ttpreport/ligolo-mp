package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/ttpreport/ligolo-mp/assets"
	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/config"
	"github.com/ttpreport/ligolo-mp/internal/events"
	"github.com/ttpreport/ligolo-mp/internal/operator"
	"github.com/ttpreport/ligolo-mp/internal/session"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type ligoloServer struct {
	pb.UnimplementedLigoloServer
	connMutex     sync.RWMutex
	connections   map[string]*ligoloConnection
	ligoloConfig  *config.Config
	sessService   *session.SessionService
	certService   *certificate.CertificateService
	operService   *operator.OperatorService
	assetsService *assets.AssetsService
}

type ligoloConnection struct {
	Stream pb.Ligolo_JoinServer
	kill   chan bool
}

func (c *ligoloConnection) Kill() <-chan bool {
	return c.kill
}

func (c *ligoloConnection) Terminate() {
	c.kill <- true
}

func NewLigoloConnection(stream pb.Ligolo_JoinServer) *ligoloConnection {
	return &ligoloConnection{
		Stream: stream,
		kill:   make(chan bool),
	}
}

func (s *ligoloServer) Join(in *pb.Empty, stream pb.Ligolo_JoinServer) error {
	slog.Debug("Received request to join")
	oper, err := s.operatorFromContext(stream.Context())
	if err != nil {
		return err
	}

	if _, ok := s.connections[oper.Name]; ok {
		return errors.New("operator already connected")
	}

	events.Publish(events.OK, "%s joined the game", oper.Name)

	s.connMutex.Lock()
	connection := NewLigoloConnection(stream)
	s.connections[oper.Name] = connection
	s.connMutex.Unlock()

	select {
	case <-stream.Context().Done():
		slog.Info("Connection gracefully closed", slog.Any("operator", oper.Name))
	case <-connection.Kill():
		slog.Info("Received request to kill connection", slog.Any("operator", oper.Name))
	}

	s.connMutex.Lock()
	delete(s.connections, oper.Name)
	s.connMutex.Unlock()

	events.Publish(events.ERROR, "%s has left the game", oper.Name)

	return nil
}

func (s *ligoloServer) HandleEvents() {
	for {
		event := events.Recv()

		pbEvent := &pb.Event{
			Type: int32(event.Type),
			Data: event.Data,
		}

		for _, connection := range s.connections {
			slog.Debug("trying to send event to operator")
			if err := connection.Stream.Send(pbEvent); err != nil {
				slog.Error("Sending event to operator failed", slog.Any("reason", err))
			}
		}
	}
}

func (s *ligoloServer) GetSessions(ctx context.Context, in *pb.Empty) (*pb.GetSessionsResp, error) {
	slog.Debug("Received request to list sessions", slog.Any("in", in))
	sessions, err := s.sessService.GetAll()
	if err != nil {
		return nil, err
	}
	result := &pb.GetSessionsResp{Sessions: []*pb.Session{}}

	for _, session := range sessions {
		result.Sessions = append(result.Sessions, session.Proto())
	}

	return result, nil
}

func (s *ligoloServer) RenameSession(ctx context.Context, in *pb.RenameSessionReq) (*pb.Empty, error) {
	slog.Debug("Received request to rename session", slog.Any("in", in))

	sess := s.sessService.GetSession(in.SessionID)
	err := s.sessService.RenameSession(in.SessionID, in.Alias)
	if err == nil {
		oper := ctx.Value("operator").(*operator.Operator)
		events.Publish(events.OK, "%s: session '%s' renamed to '%s'", oper.Name, sess.GetName(), in.Alias)
	}

	return &pb.Empty{}, err
}

func (s *ligoloServer) KillSession(ctx context.Context, in *pb.KillSessionReq) (*pb.Empty, error) {
	slog.Debug("Received request to kill session", slog.Any("in", in))

	sess := s.sessService.GetSession(in.SessionID)
	err := s.sessService.KillSession(in.SessionID)
	if err == nil {
		oper := ctx.Value("operator").(*operator.Operator)
		events.Publish(events.OK, "%s: session '%s' killed", oper.Name, sess.GetName())
	}

	return &pb.Empty{}, err
}

func (s *ligoloServer) StartRelay(ctx context.Context, in *pb.StartRelayReq) (*pb.Empty, error) {
	slog.Debug("Received request to start relay", slog.Any("in", in))

	sess := s.sessService.GetSession(in.SessionID)
	err := s.sessService.StartRelay(in.SessionID)
	if err == nil {
		oper := ctx.Value("operator").(*operator.Operator)
		events.Publish(events.OK, "%s: started relay to '%s'", oper.Name, sess.GetName())
	}

	return &pb.Empty{}, err
}

func (s *ligoloServer) StopRelay(ctx context.Context, in *pb.StopRelayReq) (*pb.Empty, error) {
	slog.Debug("Received request to stop relay", slog.Any("in", in))

	sess := s.sessService.GetSession(in.SessionID)
	err := s.sessService.StopRelay(in.SessionID)
	if err == nil {
		oper := ctx.Value("operator").(*operator.Operator)
		events.Publish(events.OK, "%s: stopped relay to '%s'", oper.Name, sess.GetName())
	}

	return &pb.Empty{}, err
}

func (s *ligoloServer) AddRoute(ctx context.Context, in *pb.AddRouteReq) (*pb.AddRouteResp, error) {
	slog.Debug("Received request to create route", slog.Any("in", in))

	if !in.Force {
		if overlappingSession, overlappingRoute := s.sessService.RouteOverlaps(in.Route.Cidr); overlappingSession != nil {
			return nil, fmt.Errorf("route overlaps with route %s in session %s", overlappingRoute, overlappingSession.GetName())
		}
	}

	sess := s.sessService.GetSession(in.SessionID)
	err := s.sessService.NewRoute(in.SessionID, in.Route.Cidr, in.Route.IsLoopback)
	if err == nil {
		oper := ctx.Value("operator").(*operator.Operator)
		if in.Route.IsLoopback {
			events.Publish(events.OK, "%s: loopback route '%s' added to '%s'", oper.Name, in.Route.Cidr, sess.GetName())
		} else {
			events.Publish(events.OK, "%s: regular route '%s' added to '%s'", oper.Name, in.Route.Cidr, sess.GetName())
		}
	}

	return &pb.AddRouteResp{}, err
}

func (s *ligoloServer) DelRoute(ctx context.Context, in *pb.DelRouteReq) (*pb.Empty, error) {
	slog.Debug("Received request to delete route", slog.Any("in", in))

	sess := s.sessService.GetSession(in.SessionID)
	err := s.sessService.RemoveRoute(in.SessionID, in.Cidr)
	if err == nil {
		oper := ctx.Value("operator").(*operator.Operator)
		events.Publish(events.OK, "%s: route '%s' removed from '%s'", oper.Name, in.Cidr, sess.GetName())
	}

	return &pb.Empty{}, err
}

func (s *ligoloServer) AddRedirector(ctx context.Context, in *pb.AddRedirectorReq) (*pb.Empty, error) {
	slog.Debug("Received request to create redirector", slog.Any("in", in))

	sess := s.sessService.GetSession(in.SessionID)
	err := s.sessService.NewRedirector(in.SessionID, in.Protocol, in.From, in.To)
	if err == nil {
		oper := ctx.Value("operator").(*operator.Operator)
		events.Publish(events.OK, "%s: redirector '%s'-->'%s' added to '%s'", oper.Name, in.From, in.To, sess.GetName())
	}

	return &pb.Empty{}, err
}

func (s *ligoloServer) DelRedirector(ctx context.Context, in *pb.DelRedirectorReq) (*pb.Empty, error) {
	slog.Debug("Received request to delete redirector", slog.Any("in", in))

	sess := s.sessService.GetSession(in.SessionID)
	redir := sess.GetRedirector(in.RedirectorID)
	err := s.sessService.RemoveRedirector(in.SessionID, in.RedirectorID)
	if err == nil {
		oper := ctx.Value("operator").(*operator.Operator)
		events.Publish(events.OK, "%s: redirector '%s'-->'%s' added to '%s'", oper.Name, redir.From, redir.To, sess.GetName())
	}

	return &pb.Empty{}, err
}

func (s *ligoloServer) GenerateAgent(ctx context.Context, in *pb.GenerateAgentReq) (*pb.GenerateAgentResp, error) {
	CACert := s.certService.GetCA()
	if CACert == nil {
		return nil, fmt.Errorf("CA certificate not found")
	}

	cert, err := s.certService.GenerateCert("", CACert)
	if err != nil {
		return nil, err
	}

	result, err := s.assetsService.CompileAgent(
		in.GOOS,
		in.GOARCH,
		in.Obfuscate,
		in.SocksServer,
		in.SocksUser,
		in.SocksPass,
		in.Servers,
		string(CACert.Certificate),
		string(cert.Certificate),
		string(cert.Key),
	)
	if err != nil {
		return nil, err
	}

	return &pb.GenerateAgentResp{AgentBinary: result}, nil
}

func (s *ligoloServer) GetOperators(ctx context.Context, in *pb.Empty) (*pb.GetOperatorsResp, error) {
	slog.Debug("Received request to list operators", slog.Any("in", in))
	oper := ctx.Value("operator").(*operator.Operator)
	if !oper.IsAdmin {
		return nil, errors.New("access denied")
	}

	opers, err := s.operService.AllOperators()
	if err != nil {
		return nil, err
	}

	var pbOpers []*pb.Operator
	for _, oper := range opers {
		pbOper := oper.Proto()

		if _, ok := s.connections[oper.Name]; ok {
			pbOper.IsOnline = true
		}

		pbOpers = append(pbOpers, pbOper)
	}

	return &pb.GetOperatorsResp{Operators: pbOpers}, nil
}

func (s *ligoloServer) ExportOperator(ctx context.Context, in *pb.ExportOperatorReq) (*pb.ExportOperatorResp, error) {
	slog.Debug("Received request to delete operator", slog.Any("in", in))
	oper := ctx.Value("operator").(*operator.Operator)
	if !oper.IsAdmin {
		return nil, errors.New("access denied")
	}

	oper, err := s.operService.OperatorByName(in.Name)
	if err != nil {
		return nil, err
	}

	config, err := oper.ToBytes()
	if err != nil {
		return nil, err
	}

	return &pb.ExportOperatorResp{
		Operator: oper.Proto(),
		Config:   config,
	}, nil
}

func (s *ligoloServer) AddOperator(ctx context.Context, in *pb.AddOperatorReq) (*pb.AddOperatorResp, error) {
	slog.Debug("Received request to create operator", slog.Any("in", in))
	oper := ctx.Value("operator").(*operator.Operator)
	if !oper.IsAdmin {
		return nil, errors.New("access denied")
	}

	if _, _, err := net.SplitHostPort(in.Operator.Server); err != nil {
		return nil, fmt.Errorf("server is malformed: %s", err)
	}

	newOperator, err := s.operService.NewOperator(in.Operator.Name, in.Operator.IsAdmin, in.Operator.Server)
	if err != nil {
		return nil, err
	}

	return &pb.AddOperatorResp{
		Operator: newOperator.Proto(),
		Credentials: &pb.OperatorCredentials{
			Cert: newOperator.Cert.Proto(),
			CA:   newOperator.CA,
		},
	}, nil
}

func (s *ligoloServer) DelOperator(ctx context.Context, in *pb.DelOperatorReq) (*pb.Empty, error) {
	slog.Debug("Received request to delete operator", slog.Any("in", in))
	oper := ctx.Value("operator").(*operator.Operator)
	if !oper.IsAdmin {
		return nil, errors.New("access denied")
	}

	opers, err := s.operService.AllOperators()
	if err != nil {
		return nil, err
	}

	if len(opers) <= 1 {
		return nil, errors.New("this is the last operator")
	}

	targetOper, err := s.operService.OperatorByName(in.Name)
	if err != nil {
		return nil, err
	}

	if targetOper.IsAdmin {
		counter := 0
		for _, oper := range opers {
			if oper.IsAdmin {
				counter++
			}
		}

		if counter <= 1 {
			return nil, errors.New("this is the last admin remaining")
		}
	}

	s.connections[targetOper.Name].Terminate()

	removedOperator, err := s.operService.RemoveOperator(in.Name)
	if err != nil {
		return nil, err
	}

	err = s.certService.Revoke(removedOperator.Cert, "removed by admin")
	if err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func (s *ligoloServer) PromoteOperator(ctx context.Context, in *pb.PromoteOperatorReq) (*pb.Empty, error) {
	slog.Debug("Received request to promote operator", slog.Any("in", in))
	oper := ctx.Value("operator").(*operator.Operator)
	if !oper.IsAdmin {
		return nil, errors.New("access denied")
	}

	_, err := s.operService.PromoteOperator(in.Name)
	if err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func (s *ligoloServer) DemoteOperator(ctx context.Context, in *pb.DemoteOperatorReq) (*pb.Empty, error) {
	slog.Debug("Received request to demote operator", slog.Any("in", in))
	oper := ctx.Value("operator").(*operator.Operator)
	if !oper.IsAdmin {
		return nil, errors.New("access denied")
	}

	opers, err := s.operService.AllOperators()
	if err != nil {
		return nil, err
	}

	counter := 0
	for _, oper := range opers {
		if oper.IsAdmin {
			counter++
		}
	}

	if counter <= 1 {
		return nil, errors.New("this is the last admin remaining")
	}

	_, err = s.operService.DemoteOperator(in.Name)
	if err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func (s *ligoloServer) GetCerts(ctx context.Context, in *pb.Empty) (*pb.GetCertsResp, error) {
	slog.Debug("Received request to list certs", slog.Any("in", in))
	oper := ctx.Value("operator").(*operator.Operator)
	if !oper.IsAdmin {
		return nil, errors.New("access denied")
	}

	certs, err := s.certService.GetAll()
	if err != nil {
		return nil, err
	}

	var pbCerts []*pb.Cert
	for _, cert := range certs {
		pbCerts = append(pbCerts, cert.Proto())
	}

	return &pb.GetCertsResp{Certs: pbCerts}, nil
}

func (s *ligoloServer) RegenCert(ctx context.Context, in *pb.RegenCertReq) (*pb.Empty, error) {
	slog.Debug("Received request to regenerate certs", slog.Any("in", in))
	oper := ctx.Value("operator").(*operator.Operator)
	if !oper.IsAdmin {
		return nil, errors.New("access denied")
	}

	_, err := s.certService.RegenerateCert(in.Name)

	return &pb.Empty{}, err
}

func (s *ligoloServer) operatorFromContext(ctx context.Context) (*operator.Operator, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("unknown error reading grpc context")
	}

	tlsInfo := p.AuthInfo.(credentials.TLSInfo)

	incomingCert := tlsInfo.State.VerifiedChains[0][0]
	operatorName := incomingCert.Subject.CommonName

	operator, err := s.operService.OperatorByName(operatorName)
	if err != nil {
		return nil, err
	}

	incomingThumbprint := s.certService.Thumbprint(incomingCert.Raw)

	if operator == nil || operator.Cert.Thumbprint != incomingThumbprint {
		return nil, errors.New("authentication failed")
	}

	return operator, nil
}

func (s *ligoloServer) certificateFromContext(ctx context.Context) (*x509.Certificate, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("unknown error reading grpc context")
	}

	tlsInfo := p.AuthInfo.(credentials.TLSInfo)

	return tlsInfo.State.VerifiedChains[0][0], nil
}

func (s *ligoloServer) unaryAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	oper, err := s.operatorFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return handler(
		context.WithValue(ctx, "operator", oper),
		req,
	)
}

func Run(config *config.Config, certService *certificate.CertificateService, sessService *session.SessionService, operService *operator.OperatorService, assetsService *assets.AssetsService) error {
	lis, err := net.Listen("tcp", config.OperatorAddr)
	if err != nil {
		return err
	}

	CACert := certService.GetCA()
	serverCert := certService.GetOperatorServerCert()

	tlsCert, err := serverCert.KeyPair()
	if err != nil {
		return err
	}

	certPool, err := CACert.CertPool()
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{tlsCert},
		ClientCAs:          certPool,
		RootCAs:            certPool,
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}

			options := x509.VerifyOptions{
				Roots: certPool,
			}
			if options.Roots == nil {
				return errors.New("no root certificate")
			}
			if _, err := cert.Verify(options); err != nil {
				return err
			}

			incomingCert := verifiedChains[0][0]
			if certService.IsRevoked(incomingCert) {
				return errors.New("certificate has been revoked")
			}

			return nil
		},
	}
	ligoloServer := &ligoloServer{
		connections:   make(map[string]*ligoloConnection),
		ligoloConfig:  config,
		sessService:   sessService,
		certService:   certService,
		operService:   operService,
		assetsService: assetsService,
	}
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)), grpc.UnaryInterceptor(ligoloServer.unaryAuthInterceptor))

	pb.RegisterLigoloServer(grpcServer, ligoloServer)
	slog.Info("server listening", slog.Any("addr", lis.Addr()))

	go ligoloServer.HandleEvents()

	return grpcServer.Serve(lis)
}
