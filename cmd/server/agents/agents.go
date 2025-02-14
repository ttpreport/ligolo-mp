package agents

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/config"
	"github.com/ttpreport/ligolo-mp/internal/events"
	"github.com/ttpreport/ligolo-mp/internal/session"
)

type AgentApiHandler struct {
	config         *config.Config
	certService    *certificate.CertificateService
	sessionService *session.SessionService

	connections chan net.Conn
	quit        chan error
}

func Run(config *config.Config, certService *certificate.CertificateService, sessionService *session.SessionService) error {
	CACert := certService.GetCA()
	certpool, err := CACert.CertPool()
	if err != nil {
		return err
	}

	agentCert := certService.GetAgentServerCert()
	tlsCert, err := agentCert.KeyPair()
	if err != nil {
		return err
	}

	handler := &AgentApiHandler{
		config:         config,
		certService:    certService,
		sessionService: sessionService,
		connections:    make(chan net.Conn, 4096),
		quit:           make(chan error, 1),
	}

	tlsConfig := &tls.Config{
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{tlsCert},
		ClientCAs:          certpool,
		RootCAs:            certpool,
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}

			options := x509.VerifyOptions{
				Roots: certpool,
			}
			if options.Roots == nil {
				return errors.New("no root certificate")
			}
			if _, err := cert.Verify(options); err != nil {
				return err
			}

			return nil
		},
	}

	go handler.serve("tcp", config.ListenInterface, tlsConfig)

	return <-handler.quit
}

func (aah *AgentApiHandler) serve(protocol string, listenIface string, tlsConfig *tls.Config) {
	defer func() { aah.quit <- nil }()

	server, err := tls.Listen(protocol, listenIface, tlsConfig)
	if err != nil {
		slog.Error("Could not start agent server",
			slog.Any("error", err),
		)
		return
	}
	defer server.Close()

	slog.Info("Agent server started",
		slog.Any("address", listenIface),
	)

	go aah.startHandler()

	for {
		conn, err := server.Accept()
		if err != nil {
			slog.Error("Agent server encountered an error",
				slog.Any("error", err),
			)

			if err == net.ErrClosed {
				return
			}

			continue
		}
		aah.connections <- conn
	}
}

func (aah *AgentApiHandler) startHandler() {
	for {
		remoteConn := <-aah.connections
		slog.Debug("agent connection received")

		yamuxConn, err := yamux.Client(remoteConn, nil)
		if err != nil {
			slog.Error("could not open multiplexed connection with agent")
			continue
		}
		slog.Debug("established multiplexed connection with agent")

		newSession, err := aah.sessionService.NewSession(yamuxConn)
		if err != nil {
			slog.Error("could not initialize new session", slog.Any("error", err))
			yamuxConn.Close()
			continue
		}
		slog.Debug("new session created", slog.Any("session", newSession))

		go aah.startSessionMonitor(newSession)

		slog.Debug("session initialized")

		events.Publish(events.OK, "new session with '%s' established", newSession.Hostname)
	}

}

func (aah *AgentApiHandler) startSessionMonitor(sess *session.Session) {
	<-sess.Multiplex.CloseChan()

	slog.Debug("session multiplexer closed", slog.Any("session", sess))
	aah.sessionService.DisconnectSession(sess.ID)
	events.Publish(events.ERROR, "session with '%s' disconnected", sess.Hostname)
}

func (aah *AgentApiHandler) Close() {
	aah.quit <- nil
}
