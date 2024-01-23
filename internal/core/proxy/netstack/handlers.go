package netstack

import (
	"io"
	"log/slog"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/adapters/gonet"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/header"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/stack"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/transport/icmp"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/transport/tcp"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/transport/udp"
	"github.com/ttpreport/gvisor-ligolo/pkg/waiter"
	"github.com/ttpreport/ligolo-mp/internal/core/protocol"
	"github.com/ttpreport/ligolo-mp/internal/core/relay"
	"github.com/ttpreport/ligolo-mp/internal/core/tuns"
)

// handleICMP process incoming ICMP packets and, depending on the target host status, respond a ICMP ECHO Reply
// Please note that other ICMP messages are not yet supported.
func handleICMP(nstack *stack.Stack, localConn TunConn, yamuxConn *yamux.Session) {
	pkt := localConn.GetICMP().Request
	v, ok := pkt.Data().PullUp(header.ICMPv4MinimumSize)
	if !ok {
		return
	}
	h := header.ICMPv4(v)
	if h.Type() == header.ICMPv4Echo {
		iph := header.IPv4(pkt.NetworkHeader().Slice())
		yamuxConnectionSession, err := yamuxConn.Open()
		if err != nil {
			slog.Error("ICMP handler encountered an error",
				slog.Any("error", err),
			)
			return
		}
		slog.Debug("Checking if destination is alive",
			slog.Any("destination", iph.DestinationAddress().String()),
		)
		icmpPacket := protocol.HostPingRequestPacket{Address: iph.DestinationAddress().String()}

		protocolEncoder := protocol.NewEncoder(yamuxConnectionSession)
		protocolDecoder := protocol.NewDecoder(yamuxConnectionSession)

		if err := protocolEncoder.Encode(protocol.Envelope{
			Type:    protocol.MessageHostPingRequest,
			Payload: icmpPacket,
		}); err != nil {
			slog.Error("ICMP handler encountered an error",
				slog.Any("error", err),
			)
			return
		}

		slog.Debug("Awaiting ping response")
		if err := protocolDecoder.Decode(); err != nil {
			slog.Error("ICMP handler encountered an error",
				slog.Any("error", err),
			)
			return
		}

		response := protocolDecoder.Envelope.Payload
		reply := response.(protocol.HostPingResponsePacket)
		if reply.Alive {
			slog.Debug("Host is alive, sending reply")
			ProcessICMP(nstack, pkt)

		}

	}
	// Ignore other ICMPs
}

func HandlePacket(nstack *stack.Stack, localConn TunConn, yamuxConn *yamux.Session, tun *tuns.Tun) {

	var endpointID stack.TransportEndpointID
	var prototransport uint8
	var protonet uint8

	// Switching part
	switch localConn.Protocol {
	case tcp.ProtocolNumber:
		endpointID = localConn.GetTCP().EndpointID
		prototransport = protocol.TransportTCP
	case udp.ProtocolNumber:
		endpointID = localConn.GetUDP().EndpointID
		prototransport = protocol.TransportUDP
	case icmp.ProtocolNumber4:
		// ICMPs can't be relayed
		handleICMP(nstack, localConn, yamuxConn)
		return
	}

	if endpointID.LocalAddress.To4() != (tcpip.Address{}) {
		protonet = protocol.Networkv4
	} else {
		protonet = protocol.Networkv6
	}

	slog.Debug("Received packet",
		slog.Any("remote-address", endpointID.RemoteAddress),
		slog.Any("remote-port", endpointID.RemotePort),
		slog.Any("local-address", endpointID.LocalAddress),
		slog.Any("local-port", endpointID.LocalPort),
	)

	yamuxConnectionSession, err := yamuxConn.Open()
	if err != nil {
		slog.Error("Packet handler encountered an error",
			slog.Any("error", err),
		)
		return
	}

	address := endpointID.LocalAddress.String()
	if tun.IsLoopback {
		address = "127.0.0.1"
	}

	connectPacket := protocol.ConnectRequestPacket{
		Net:       protonet,
		Transport: prototransport,
		Address:   address,
		Port:      endpointID.LocalPort,
	}

	protocolEncoder := protocol.NewEncoder(yamuxConnectionSession)
	protocolDecoder := protocol.NewDecoder(yamuxConnectionSession)

	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageConnectRequest,
		Payload: connectPacket,
	}); err != nil {
		slog.Error("Packet handler encountered an error",
			slog.Any("error", err),
		)
		return
	}

	slog.Debug("Awaiting response")
	if err := protocolDecoder.Decode(); err != nil {
		if err != io.EOF {
			slog.Error("Packet handler encountered an error",
				slog.Any("error", err),
			)
		}
		return
	}

	response := protocolDecoder.Envelope.Payload
	reply := response.(protocol.ConnectResponsePacket)
	if reply.Established {
		slog.Debug("Connection established on remote end")
		go func() {
			var wq waiter.Queue
			if localConn.IsTCP() {
				ep, iperr := localConn.GetTCP().Request.CreateEndpoint(&wq)
				if iperr != nil {
					slog.Error("Packet handler encountered an error",
						slog.Any("error", iperr),
					)
					localConn.Terminate(true)
					return
				}
				gonetConn := gonet.NewTCPConn(&wq, ep)
				go relay.StartRelay(yamuxConnectionSession, gonetConn)

			} else if localConn.IsUDP() {
				ep, iperr := localConn.GetUDP().Request.CreateEndpoint(&wq)
				if iperr != nil {
					slog.Error("Packet handler encountered an error",
						slog.Any("error", iperr),
					)
					localConn.Terminate(false)
					return
				}

				gonetConn := gonet.NewUDPConn(nstack, &wq, ep)
				go relay.StartRelay(yamuxConnectionSession, gonetConn)
			}

		}()
	} else {
		localConn.Terminate(reply.Reset)

	}

}
