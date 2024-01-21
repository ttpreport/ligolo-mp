package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"

	"github.com/ttpreport/ligolo-mp/internal/common/config"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
)

type Controller struct {
	Network        string
	Connection     chan net.Conn
	startchan      chan interface{}
	certificateMap map[string]*tls.Certificate
	ControllerConfig
}

type ControllerConfig struct {
	Address     string
	Certificate tls.Certificate
	CertPool    *x509.CertPool
	CA          []byte
}

func New(config *config.Config, store *storage.Store) Controller {
	dbcacert, err := store.GetCert("_ca_")
	if err != nil {
		panic(err)
	}
	certpool := x509.NewCertPool()
	if ok := certpool.AppendCertsFromPEM(dbcacert.Certificate); !ok {
		panic("malformed CA certificate")
	}

	dbservercert, err := store.GetCert("_agent_")
	if err != nil {
		panic(err)
	}
	tlsCert, _ := tls.X509KeyPair(dbservercert.Certificate, dbservercert.Key)

	controllerConfig := ControllerConfig{
		Address:     config.ListenInterface,
		CertPool:    certpool,
		Certificate: tlsCert,
	}
	return Controller{Network: "tcp", Connection: make(chan net.Conn, 4096), ControllerConfig: controllerConfig, startchan: make(chan interface{}), certificateMap: make(map[string]*tls.Certificate)}
}

func (c *Controller) ListenAndServe() {
	defer func() { <-c.startchan }()

	go func() {
		tlsConfig := &tls.Config{
			ClientAuth:         tls.RequireAndVerifyClientCert,
			Certificates:       []tls.Certificate{c.Certificate},
			ClientCAs:          c.CertPool,
			RootCAs:            c.CertPool,
			MinVersion:         tls.VersionTLS13,
			MaxVersion:         tls.VersionTLS13,
			InsecureSkipVerify: true,
			VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				cert, err := x509.ParseCertificate(rawCerts[0])
				if err != nil {
					return err
				}

				options := x509.VerifyOptions{
					Roots: c.CertPool,
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

		listener, err := tls.Listen(c.Network, c.Address, tlsConfig)
		if err != nil {
			panic(err)
		}
		defer listener.Close()
		close(c.startchan) // Controller is listening.
		slog.Debug("Controller started",
			slog.Any("address", c.Address),
		)
		for {
			conn, err := listener.Accept()
			if err != nil {
				slog.Error("Listener encountered an error",
					slog.Any("error", err),
				)
				continue
			}
			c.Connection <- conn
		}
	}()
}
