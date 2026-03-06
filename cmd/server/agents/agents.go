package agents

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/v2/internal/certificate"
	"github.com/ttpreport/ligolo-mp/v2/internal/config"
	"github.com/ttpreport/ligolo-mp/v2/internal/events"
	"github.com/ttpreport/ligolo-mp/v2/internal/session"
)

type AgentApiHandler struct {
	config         *config.Config
	certService    *certificate.CertificateService
	sessionService *session.SessionService

	connections chan net.Conn
	quit        chan error
}

func Run(config *config.Config, certService *certificate.CertificateService, sessionService *session.SessionService) error {
	CACert, err := certService.GetCA()
	if err != nil {
		return err
	}
	if CACert == nil {
		return errors.New("CA certificate not found")
	}
	certpool, err := CACert.CertPool()
	if err != nil {
		return err
	}

	agentCert, err := certService.GetAgentServerCert()
	if err != nil {
		return err
	}
	if agentCert == nil {
		return errors.New("agent server certificate not found")
	}
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

	var clientAuth = tls.RequireAndVerifyClientCert
	if config.InsecureAgents {
		clientAuth = tls.NoClientCert
	}

	tlsConfig := &tls.Config{
		ClientAuth:         clientAuth,
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

	err = aah.sessionService.CleanUp()
	if err != nil {
		slog.Error("Could not clean up sessions",
			slog.Any("error", err),
		)
	}

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

		config := yamux.DefaultConfig()
		config.LogOutput = io.Discard
		yamuxConn, err := yamux.Client(remoteConn, config)
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

		events.Publish(events.OK, "new session with '%s' established", newSession.GetName())
	}

}

func (aah *AgentApiHandler) startSessionMonitor(sess *session.Session) {
	tick := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-tick.C:
			aah.sessionService.UpdateLastSeen(sess.ID)
		case <-sess.Multiplex.CloseChan():
			slog.Debug("session multiplexer closed", slog.Any("session", sess))
			aah.sessionService.DisconnectSession(sess.ID)
			events.Publish(events.ERROR, "session with '%s' disconnected", sess.GetName())
			return
		}
	}

}

func (aah *AgentApiHandler) Close() {
	aah.quit <- nil
}
