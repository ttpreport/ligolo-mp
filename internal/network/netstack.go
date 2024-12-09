package network

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/gvisor-ligolo/pkg/buffer"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/adapters/gonet"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/checksum"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/header"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/network/ipv4"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/network/ipv6"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/stack"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/transport/icmp"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/transport/raw"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/transport/tcp"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/transport/udp"
	"github.com/ttpreport/gvisor-ligolo/pkg/waiter"
	"github.com/ttpreport/ligolo-mp/internal/netstack/tun"
	"github.com/ttpreport/ligolo-mp/internal/protocol"
	"github.com/ttpreport/ligolo-mp/internal/relay"
	"golang.org/x/sys/unix"
)

type TunConn struct {
	Protocol tcpip.TransportProtocolNumber
	Handler  interface{}
}

// IsTCP check if the current TunConn is TCP
func (t TunConn) IsTCP() bool {
	return t.Protocol == tcp.ProtocolNumber
}

// GetTCP returns the handler as a TCPConn
func (t TunConn) GetTCP() TCPConn {
	return t.Handler.(TCPConn)
}

// IsUDP check if the current TunConn is UDP
func (t TunConn) IsUDP() bool {
	return t.Protocol == udp.ProtocolNumber
}

// GetUDP returns the handler as a UDPConn
func (t TunConn) GetUDP() UDPConn {
	return t.Handler.(UDPConn)
}

// IsICMP check if the current TunConn is ICMP
func (t TunConn) IsICMP() bool {
	return t.Protocol == icmp.ProtocolNumber4
}

// GetICMP returns the handler as a ICMPConn
func (t TunConn) GetICMP() ICMPConn {
	return t.Handler.(ICMPConn)
}

// Terminate is call when connections need to be terminated. For now, this is only useful for TCP connections
func (t TunConn) Terminate(reset bool) {
	if t.IsTCP() {
		t.GetTCP().Request.Complete(reset)
	}
}

// TCPConn represents a TCP Forwarder connection
type TCPConn struct {
	EndpointID stack.TransportEndpointID
	Request    *tcp.ForwarderRequest
}

// UDPConn represents a UDP Forwarder connection
type UDPConn struct {
	EndpointID stack.TransportEndpointID
	Request    *udp.ForwarderRequest
}

// ICMPConn represents a ICMP Packet Buffer
type ICMPConn struct {
	Request stack.PacketBufferPtr
}

// NetStack is the structure used to store the connection pool and the gvisor network stack
type NetStack struct {
	pool  *ConnPool
	stack *stack.Stack
	sync.Mutex
	closeChan chan bool
	fd        int
}

// GetStack returns the current Gvisor stack.Stack object
func (s *NetStack) GetStack() *stack.Stack {
	return s.stack
}

// SetConnPool is used to change the current connPool. It must be used after switching Ligolo agents
func (s *NetStack) SetConnPool(connPool *ConnPool) {
	s.Lock()
	s.pool = connPool
	s.Unlock()
}

// Cleans up after gVisor. Couldn't find a better way
func (s *NetStack) Destroy() error {
	s.closeChan <- true

	if err := unix.Close(s.fd); err != nil {
		return err
	}

	s.stack.Destroy()

	return nil
}

func (s *NetStack) ClosePool() <-chan interface{} {
	return s.pool.CloseChan
}

func (s *NetStack) GetTunConn() <-chan TunConn {
	return s.pool.Pool
}

func (ns *NetStack) HandlePacket(localConn TunConn, multiplex *yamux.Session, localRoutes []Route) {
	var endpointID stack.TransportEndpointID
	var prototransport uint8
	var protonet uint8

	switch localConn.Protocol {
	case tcp.ProtocolNumber:
		endpointID = localConn.GetTCP().EndpointID
		prototransport = protocol.TransportTCP
	case udp.ProtocolNumber:
		endpointID = localConn.GetUDP().EndpointID
		prototransport = protocol.TransportUDP
	case icmp.ProtocolNumber4:
		// ICMPs can't be relayed
		ns.handleICMP(localConn, multiplex, localRoutes)
		return
	}

	if endpointID.LocalAddress.To4() != (tcpip.Address{}) {
		protonet = protocol.Networkv4
	} else {
		protonet = protocol.Networkv6
	}

	yamuxConnectionSession, err := multiplex.Open()
	if err != nil {
		slog.Error("Packet handler encountered an error",
			slog.Any("error", err),
		)
		return
	}

	address := endpointID.LocalAddress.String()
	for _, localRoute := range localRoutes {
		ip := net.ParseIP(address)
		if localRoute.Cidr.Contains(ip) {
			if protonet == protocol.Networkv4 {
				address = "127.0.0.1"
			} else {
				address = "::1"
			}

			break
		}
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

				gonetConn := gonet.NewUDPConn(ns.stack, &wq, ep)
				go relay.StartRelay(yamuxConnectionSession, gonetConn)
			}

		}()
	} else {
		localConn.Terminate(reply.Reset)
	}

}

// icmpResponder handle ICMP packets coming to gvisor/netstack.
// Instead of responding to all ICMPs ECHO by default, we try to
// execute a ping on the Agent, and depending of the response, we
// send a ICMP reply back.
func (ns *NetStack) icmpResponder() (chan bool, error) {
	quit := make(chan bool)
	var wq waiter.Queue
	rawProto, rawerr := raw.NewEndpoint(ns.stack, ipv4.ProtocolNumber, icmp.ProtocolNumber4, &wq)
	if rawerr != nil {
		return nil, errors.New("could not create raw endpoint")
	}
	if err := rawProto.Bind(tcpip.FullAddress{}); err != nil {
		return nil, errors.New("could not bind raw endpoint")
	}
	go func() {
		defer rawProto.Close()

		we, ch := waiter.NewChannelEntry(waiter.ReadableEvents)
		wq.EventRegister(&we)

		defer wq.EventUnregister(&we)

		for {
			var buff bytes.Buffer
			_, err := rawProto.Read(&buff, tcpip.ReadOptions{})

			if _, ok := err.(*tcpip.ErrWouldBlock); ok {
				// Wait for data to become available.
				select {
				case <-quit:
					return
				case <-ch:
					_, err := rawProto.Read(&buff, tcpip.ReadOptions{})

					if err != nil {
						if _, ok := err.(*tcpip.ErrWouldBlock); ok {
							// Oh, a race condition?
							continue
						} else {
							// This is bad.
							panic(err)
						}
					}

					iph := header.IPv4(buff.Bytes())

					hlen := int(iph.HeaderLength())
					if buff.Len() < hlen {
						return
					}

					// Reconstruct a ICMP PacketBuffer from bytes.

					view := buffer.MakeWithData(buff.Bytes())
					packetbuff := stack.NewPacketBuffer(stack.PacketBufferOptions{
						Payload:            view,
						ReserveHeaderBytes: hlen,
					})

					packetbuff.NetworkProtocolNumber = ipv4.ProtocolNumber
					packetbuff.TransportProtocolNumber = icmp.ProtocolNumber4
					packetbuff.NetworkHeader().Consume(hlen)
					tunConn := TunConn{
						Protocol: icmp.ProtocolNumber4,
						Handler:  ICMPConn{Request: packetbuff},
					}

					ns.Lock()
					if ns.pool == nil || ns.pool.Closed() {
						ns.Unlock()
						continue // If connPool is closed, ignore packet.
					}

					if err := ns.pool.Add(tunConn); err != nil {
						ns.Unlock()
						slog.Error("ICMP responder encountered an error",
							slog.Any("error", err),
						)
						continue // Unknown error, continue...
					}
					ns.Unlock()
				}
			}

		}
	}()
	return quit, nil
}

// handleICMP process incoming ICMP packets and, depending on the target host status, respond a ICMP ECHO Reply
// Please note that other ICMP messages are not yet supported.
func (ns *NetStack) handleICMP(localConn TunConn, multiplex *yamux.Session, localRoutes []Route) {
	pkt := localConn.GetICMP().Request
	v, ok := pkt.Data().PullUp(header.ICMPv4MinimumSize)
	if !ok {
		return
	}
	h := header.ICMPv4(v)
	if h.Type() == header.ICMPv4Echo {
		iph := header.IPv4(pkt.NetworkHeader().Slice())

		address := iph.DestinationAddress().String()
		for _, localRoute := range localRoutes {
			ip := net.ParseIP(address)
			if localRoute.Cidr.Contains(ip) {
				address = "127.0.0.1"
				break
			}
		}

		yamuxConnectionSession, err := multiplex.Open()
		if err != nil {
			slog.Error("ICMP handler encountered an error",
				slog.Any("error", err),
			)
			return
		}
		slog.Debug("Checking if destination is alive",
			slog.Any("destination", address),
		)
		icmpPacket := protocol.HostPingRequestPacket{Address: address}

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
			ns.ProcessICMP(pkt)

		}

	}
	// Ignore other ICMPs
}

// ProcessICMP send back a ICMP echo reply from after receiving a echo request.
// This code come mostly from pkg/tcpip/network/ipv4/icmp.go
func (ns *NetStack) ProcessICMP(pkt stack.PacketBufferPtr) {
	// (gvisor) pkg/tcpip/network/ipv4/icmp.go:174 - handleICMP

	// ICMP packets don't have their TransportHeader fields set. See
	// icmp/protocol.go:protocol.Parse for a full explanation.
	v, ok := pkt.Data().PullUp(header.ICMPv4MinimumSize)
	if !ok {
		return
	}
	h := header.ICMPv4(v)
	// Ligolo-ng: not sure why, but checksum is invalid here.
	/*
		// Only do in-stack processing if the checksum is correct.
		if checksum.Checksum(h, pkt.Data().Checksum()) != 0xffff {
			return
		}
	*/
	iph := header.IPv4(pkt.NetworkHeader().Slice())
	var newOptions header.IPv4Options

	// TODO(b/112892170): Meaningfully handle all ICMP types.
	switch h.Type() {
	case header.ICMPv4Echo:
		replyData := stack.PayloadSince(pkt.TransportHeader())
		defer replyData.Release()
		ipHdr := header.IPv4(pkt.NetworkHeader().Slice())

		localAddressBroadcast := pkt.NetworkPacketInfo.LocalAddressBroadcast

		// It's possible that a raw socket expects to receive this.
		pkt = nil

		// Take the base of the incoming request IP header but replace the options.
		replyHeaderLength := uint8(header.IPv4MinimumSize + len(newOptions))
		replyIPHdrView := buffer.NewView(int(replyHeaderLength))
		replyIPHdrView.Write(iph[:header.IPv4MinimumSize])
		replyIPHdrView.Write(newOptions)
		replyIPHdr := header.IPv4(replyIPHdrView.AsSlice())
		replyIPHdr.SetHeaderLength(replyHeaderLength)

		// As per RFC 1122 section 3.2.1.3, when a host sends any datagram, the IP
		// source address MUST be one of its own IP addresses (but not a broadcast
		// or multicast address).
		localAddr := ipHdr.DestinationAddress()
		if localAddressBroadcast || header.IsV4MulticastAddress(localAddr) {
			localAddr = tcpip.Address{}
		}

		r, err := ns.stack.FindRoute(1, localAddr, ipHdr.SourceAddress(), ipv4.ProtocolNumber, false /* multicastLoop */)
		if err != nil {
			// If we cannot find a route to the destination, silently drop the packet.
			return
		}
		defer r.Release()

		replyIPHdr.SetSourceAddress(r.LocalAddress())
		replyIPHdr.SetDestinationAddress(r.RemoteAddress())
		replyIPHdr.SetTTL(r.DefaultTTL())

		replyICMPHdr := header.ICMPv4(replyData.AsSlice())
		replyICMPHdr.SetType(header.ICMPv4EchoReply)
		replyICMPHdr.SetChecksum(0)
		replyICMPHdr.SetChecksum(^checksum.Checksum(replyData.AsSlice(), 0))

		replyBuf := buffer.MakeWithView(replyIPHdrView)
		replyBuf.Append(replyData.Clone())
		replyPkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			ReserveHeaderBytes: int(r.MaxHeaderLength()),
			Payload:            replyBuf,
		})

		replyPkt.TransportProtocolNumber = header.ICMPv4ProtocolNumber

		if err := r.WriteHeaderIncludedPacket(replyPkt); err != nil {
			panic(err)
		}
	}
}

type ConnPool struct {
	CloseChan chan interface{}
	Pool      chan TunConn
	sync.Mutex
}

func NewConnPool(size int) *ConnPool {
	return &ConnPool{CloseChan: make(chan interface{}), Pool: make(chan TunConn, size)}
}
func (p *ConnPool) Add(packet TunConn) error {
	p.Lock()
	defer p.Unlock()

	select {
	case <-p.CloseChan:
		return errors.New("pool is closed")
	default:
		p.Pool <- packet
	}
	return nil
}

func (p *ConnPool) Close() error {
	p.Lock()
	defer p.Unlock()

	select {
	case <-p.CloseChan:
		return errors.New("pool is already closed")
	default:
		close(p.CloseChan)
		close(p.Pool)
		p.Pool = nil
	}
	return nil
}

func (p *ConnPool) Closed() bool {
	select {
	case <-p.CloseChan:
		return true
	default:
		return false
	}
}

func (p *ConnPool) Get() (TunConn, error) {
	p.Lock()
	defer p.Unlock()
	select {
	case <-p.CloseChan:
		return TunConn{}, errors.New("pool is closed")
	case tunconn := <-p.Pool:
		return tunconn, nil
	}
}

func NewNetstack(maxConnections int, maxInFlight int, tunName string) (*NetStack, error) {
	connPool := NewConnPool(maxConnections)
	ns := &NetStack{
		pool: connPool,
	}
	ns.stack = stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
			icmp.NewProtocol6,
		},
		HandleLocal: false,
	})

	// Gvisor Hack: Disable ICMP handling.
	ns.stack.SetICMPLimit(0)
	ns.stack.SetICMPBurst(0)

	// Forward TCP connections
	tcpHandler := tcp.NewForwarder(ns.stack, 0, maxInFlight, func(request *tcp.ForwarderRequest) {
		tcpConn := TCPConn{
			EndpointID: request.ID(),
			Request:    request,
		}

		ns.Lock()
		defer ns.Unlock()
		if ns.pool == nil || ns.pool.Closed() {
			return // If connPool is closed, ignore packet.
		}

		if err := ns.pool.Add(TunConn{
			tcp.ProtocolNumber,
			tcpConn,
		}); err != nil {
			slog.Error("Netstack encountered an error", slog.Any("error", err))
		}
	})

	// Forward UDP connections
	udpHandler := udp.NewForwarder(ns.stack, func(request *udp.ForwarderRequest) {
		udpConn := UDPConn{
			EndpointID: request.ID(),
			Request:    request,
		}

		ns.Lock()
		defer ns.Unlock()

		if ns.pool == nil || ns.pool.Closed() {
			return // If connPool is closed, ignore packet.
		}

		if err := ns.pool.Add(TunConn{
			udp.ProtocolNumber,
			udpConn,
		}); err != nil {
			slog.Error("Netstack encountered an error", slog.Any("error", err))
		}
	})

	// Register forwarders
	ns.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpHandler.HandlePacket)
	ns.stack.SetTransportProtocolHandler(udp.ProtocolNumber, udpHandler.HandlePacket)

	linkEP, fd, err := tun.Open(tunName)
	if err != nil {
		return nil, err
	}

	ns.fd = fd

	// Create a new NIC
	if err := ns.stack.CreateNIC(1, linkEP); err != nil {
		return nil, errors.New(err.String())
	}

	// Start a endpoint that will reply to ICMP echo queries
	closeChan, err := ns.icmpResponder()
	if err != nil {
		return nil, err
	}

	ns.closeChan = closeChan

	// Allow all routes by default
	ns.stack.SetRouteTable([]tcpip.Route{
		{
			Destination: header.IPv4EmptySubnet,
			NIC:         1,
		},
		{
			Destination: header.IPv6EmptySubnet,
			NIC:         1,
		},
	})

	// Enable forwarding
	ns.stack.SetForwardingDefaultAndAllNICs(ipv4.ProtocolNumber, false)
	ns.stack.SetForwardingDefaultAndAllNICs(ipv6.ProtocolNumber, false)

	// Enable TCP SACK
	nsacks := tcpip.TCPSACKEnabled(false)
	ns.stack.SetTransportProtocolOption(tcp.ProtocolNumber, &nsacks)

	// Disable SYN-Cookies, as this can mess with nmap scans
	synCookies := tcpip.TCPAlwaysUseSynCookies(false)
	ns.stack.SetTransportProtocolOption(tcp.ProtocolNumber, &synCookies)

	// Allow packets from all sources/destinations
	ns.stack.SetPromiscuousMode(1, true)
	ns.stack.SetSpoofing(1, true)

	return ns, nil
}
