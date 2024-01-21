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
	"syscall"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/core/agents/neterror"
	"github.com/ttpreport/ligolo-mp/internal/core/agents/smartping"
	"github.com/ttpreport/ligolo-mp/internal/core/assets/agent"
	"github.com/ttpreport/ligolo-mp/internal/core/protocol"
	"github.com/ttpreport/ligolo-mp/internal/core/relay"
	goproxy "golang.org/x/net/proxy"
)

var (
	listenerMap map[int32]agent.Listener
	listenerID  int32
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
	var serverAddr = `{{ .Server }}`
	var AgentCert = []byte(`{{ .AgentCert }}`)
	var AgentKey = []byte(`{{ .AgentKey }}`)
	var CACert = []byte(`{{ .CACert }}`)

	if serverAddr == "" {
		panic("missing server addr")
	}
	host, _, err := net.SplitHostPort(serverAddr)
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

	var conn net.Conn

	listenerMap = make(map[int32]agent.Listener)

	for {
		var err error
		if socksProxy != "" {
			if _, _, err := net.SplitHostPort(socksProxy); err != nil {
				panic("invalid socks5 address")
			}
			conn, err = sockDial(serverAddr, socksProxy, socksUser, socksPass)
		} else {
			conn, err = net.Dial("tcp", serverAddr)
		}
		if err == nil {
			connect(conn, &tlsConfig)
		}

		time.Sleep(5 * time.Second)
	}
}

func sockDial(serverAddr string, socksProxy string, socksUser string, socksPass string) (net.Conn, error) {
	proxyDialer, err := goproxy.SOCKS5("tcp", socksProxy, &goproxy.Auth{
		User:     socksUser,
		Password: socksPass,
	}, goproxy.Direct)
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
			Name:       fmt.Sprintf("%s@%s", username, hostname),
			Hostname:   hostname,
			Interfaces: protocol.NewNetInterfaces(nonloopbackIfaces),
			Listeners:  protocol.NewListenerInterface(listenerMap),
		}

		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageInfoReply,
			Payload: infoResponse,
		})
	case protocol.MessageListenerCloseRequest:
		closeRequest := e.(protocol.ListenerCloseRequestPacket)
		encoder := protocol.NewEncoder(conn)

		var err error
		if lis, ok := listenerMap[closeRequest.ListenerID]; ok {
			err = lis.Close()
		} else {
			err = errors.New("invalid listener id")
		}

		listenerResponse := protocol.ListenerCloseResponsePacket{
			Err: err != nil,
		}
		if err != nil {
			listenerResponse.ErrString = err.Error()
		}

		delete(listenerMap, closeRequest.ListenerID)

		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageListenerCloseResponse,
			Payload: listenerResponse,
		})

	case protocol.MessageListenerRequest:
		listenRequest := e.(protocol.ListenerRequestPacket)
		encoder := protocol.NewEncoder(conn)

		listener, err := agent.NewListener(listenRequest.Network, listenRequest.From, listenRequest.To)
		if err != nil {
			listenerResponse := protocol.ListenerResponsePacket{
				ListenerID: 0,
				Err:        true,
				ErrString:  err.Error(),
			}
			encoder.Encode(protocol.Envelope{
				Type:    protocol.MessageListenerResponse,
				Payload: listenerResponse,
			})
			return
		}

		listenerResponse := protocol.ListenerResponsePacket{
			ListenerID: listenerID,
			Err:        false,
			ErrString:  "",
		}
		listenerMap[listenerID] = listener
		listenerID++

		encoder.Encode(protocol.Envelope{
			Type:    protocol.MessageListenerResponse,
			Payload: listenerResponse,
		})

		go listener.ListenAndRelay()
	case protocol.MessageClose:
		os.Exit(0)

	}
}
