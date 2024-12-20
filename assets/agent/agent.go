package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/netstack/neterror"
	"github.com/ttpreport/ligolo-mp/internal/netstack/smartping"
	"github.com/ttpreport/ligolo-mp/internal/protocol"
	"github.com/ttpreport/ligolo-mp/internal/relay"
	goproxy "golang.org/x/net/proxy"
)

var (
	redirectorMap map[string]relay.Redirector
)

func main() {
	defer func() { // probably okay
		if err := recover(); err != nil {
			main()
		}
	}()

	var tlsConfig tls.Config
	var socksProxy = `{{ .SocksServer }}`
	var socksUser = `{{ .SocksUser }}`
	var socksPass = `{{ .SocksPass }}`
	var servers = strings.Split(`{{ .Servers }}`, "\n")
	var AgentCert = []byte(`{{ .AgentCert }}`)
	var AgentKey = []byte(`{{ .AgentKey }}`)
	var CACert = []byte(`{{ .CACert }}`)

	var conn net.Conn
	redirectorMap = make(map[string]relay.Redirector)

	for {
		for _, server := range servers {
			host, _, err := net.SplitHostPort(server)
			if err != nil {
				panic("invalid server addr")
			}

			ca := x509.NewCertPool()
			if ok := ca.AppendCertsFromPEM(CACert); !ok {
				panic("failed to parse CACert")
			}

			mtlsCert, err := tls.X509KeyPair(AgentCert, AgentKey)
			if err != nil {
				panic(err)
			}

			tlsConfig = tls.Config{
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
					if _, err := cert.Verify(options); err != nil {
						return err
					}

					return nil
				},
			}

			if socksProxy != "" {
				if _, _, err := net.SplitHostPort(socksProxy); err != nil {
					panic("invalid socks5 address")
				}
				conn, err = sockDial(server, socksProxy, socksUser, socksPass)
			} else {
				conn, err = net.DialTimeout("tcp", server, 5*time.Second)
			}

			if err == nil {
				connect(conn, &tlsConfig)
				break
			}
		}

		time.Sleep(5 * time.Second)
	}
}

func sockDial(serverAddr string, socksProxy string, socksUser string, socksPass string) (net.Conn, error) {
	proxyDialer, err := goproxy.SOCKS5("tcp", socksProxy, &goproxy.Auth{
		User:     socksUser,
		Password: socksPass,
	}, goproxy.Direct)

	goproxy.FromEnvironment()
	if err != nil {
		panic(err)
	}
	return proxyDialer.Dial("tcp", serverAddr)
}

func connect(conn net.Conn, config *tls.Config) error {
	tlsConn := tls.Client(conn, config)

	yamuxConn, err := yamux.Server(tlsConn, yamux.DefaultConfig())
	if err != nil {
		return err
	}

	for {
		conn, err := yamuxConn.Accept()
		if err != nil {
			return err
		}
		go handleConn(conn) // this is leaking, but should be okay
	}
}

func handleConn(conn net.Conn) {
	decoder := protocol.NewDecoder(conn)
	if err := decoder.Decode(); err != nil {
		panic(err)
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

		var d net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		targetConn, err := d.DialContext(ctx, network, fmt.Sprintf("%s:%d", connRequest.Address, connRequest.Port))
		defer cancel()

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
		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageConnectResponse,
			Payload: connectPacket,
		})
		if connectPacket.Established {
			relay.StartRelay(targetConn, conn)
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

		infoResponse := protocol.InfoReplyPacket{
			Name:        fmt.Sprintf("%s@%s", username, hostname),
			Hostname:    hostname,
			Interfaces:  protocol.NewNetInterfaces(nonloopbackIfaces),
			Redirectors: protocol.NewRedirectorInterface(redirectorMap),
		}

		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageInfoReply,
			Payload: infoResponse,
		})
	case protocol.MessageRedirectorCloseRequest:
		closeRequest := e.(protocol.RedirectorCloseRequestPacket)
		encoder := protocol.NewEncoder(conn)

		var err error
		if lis, ok := redirectorMap[closeRequest.ID]; ok {
			err = lis.Close()
		}

		redirectorResponse := protocol.RedirectorCloseResponsePacket{
			Err: err != nil,
		}
		if err != nil {
			redirectorResponse.ErrString = err.Error()
		}

		delete(redirectorMap, closeRequest.ID)

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
			if _, exists := redirectorMap[redirector.ID]; exists {
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
				redirectorMap[redirector.ID] = redirector

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
