package netstack

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/v2/internal/netstack/tun"
	"github.com/ttpreport/ligolo-mp/v2/internal/protocol"
	"github.com/ttpreport/ligolo-mp/v2/internal/relay"
	"github.com/ttpreport/ligolo-mp/v2/internal/route"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
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
	Request stack.PacketBuffer
}

// NetStack is the structure used to store the connection pool and the gvisor network stack
type NetStack struct {
	pool  *ConnPool
	stack *stack.Stack
	sync.Mutex
}

// icmpEchoInterceptor implements stack.NetworkDispatcher. It sits between
// the fdbased link endpoint and gVisor's network layer. Any ICMPv4 echo
// request is diverted to the NetStack connection pool (so handleICMP can
// decide whether the remote host is alive) and is NOT forwarded to gVisor.
// This prevents gVisor's built-in ICMP echo handler from auto-replying
// to every request regardless of whether the target host actually exists.
type icmpEchoInterceptor struct {
	ns         *NetStack
	dispatcher stack.NetworkDispatcher
}

func (i *icmpEchoInterceptor) DeliverNetworkPacket(protocol tcpip.NetworkProtocolNumber, pkt *stack.PacketBuffer) {
	if protocol == ipv4.ProtocolNumber {
		// At this point the IPv4 header has NOT been consumed yet —
		// fdbased hands us the raw IP packet with everything in pkt.Data().
		// Read the minimum IPv4 header to get the transport protocol and
		// the actual header length (options may extend it past 20 bytes).
		hdrBytes, ok := pkt.Data().PullUp(header.IPv4MinimumSize)
		if ok {
			iph := header.IPv4(hdrBytes)
			if iph.TransportProtocol() == header.ICMPv4ProtocolNumber {
				hlen := int(iph.HeaderLength())
				icmpBytes, ok := pkt.Data().PullUp(hlen + header.ICMPv4MinimumSize)
				if ok && header.ICMPv4(icmpBytes[hlen:]).Type() == header.ICMPv4Echo {
					cloned := pkt.Clone()
					cloned.NetworkProtocolNumber = ipv4.ProtocolNumber
					cloned.TransportProtocolNumber = icmp.ProtocolNumber4
					// Consume the IPv4 header on the clone so that
					// handleICMP / ProcessICMP can access it via
					// pkt.NetworkHeader().Slice(), matching the layout
					// expected by those functions.
					cloned.NetworkHeader().Consume(hlen)
					tunConn := TunConn{
						Protocol: icmp.ProtocolNumber4,
						Handler:  ICMPConn{Request: *cloned},
					}
					i.ns.tryAddICMP(tunConn)
					return // suppress gVisor auto-reply
				}
			}
		}
	}
	i.dispatcher.DeliverNetworkPacket(protocol, pkt)
}

func (i *icmpEchoInterceptor) DeliverLinkPacket(protocol tcpip.NetworkProtocolNumber, pkt *stack.PacketBuffer) {
	i.dispatcher.DeliverLinkPacket(protocol, pkt)
}

// interceptingEndpoint wraps a LinkEndpoint and injects icmpEchoInterceptor
// as the NetworkDispatcher so that ICMP echo requests never reach gVisor's
// IPv4 handler.
type interceptingEndpoint struct {
	stack.LinkEndpoint
	ns *NetStack
}

func (e *interceptingEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.LinkEndpoint.Attach(&icmpEchoInterceptor{ns: e.ns, dispatcher: dispatcher})
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
	s.pool.Close()
	s.stack.Destroy()

	return nil
}

func (s *NetStack) ClosePool() <-chan interface{} {
	return s.pool.CloseChan
}

func (s *NetStack) GetTunConn() <-chan TunConn {
	return s.pool.Pool
}

func (ns *NetStack) HandlePacket(localConn TunConn, multiplex *yamux.Session, localRoutes []route.Route) {
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

	yamuxConnectionSession, err := multiplex.Open()
	if err != nil {
		slog.Debug("Packet handler encountered an error #1",
			slog.Any("error", err),
		)
		return
	}
	defer yamuxConnectionSession.Close()

	protocolEncoder := protocol.NewEncoder(yamuxConnectionSession)
	protocolDecoder := protocol.NewDecoder(yamuxConnectionSession)

	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageConnectRequest,
		Payload: connectPacket,
	}); err != nil {
		slog.Debug("Packet handler encountered an error #2",
			slog.Any("error", err),
		)
		return
	}

	if err := protocolDecoder.Decode(); err != nil {
		if err != io.EOF {
			slog.Debug("Packet handler encountered an error #3",
				slog.Any("error", err),
			)
		}
		return
	}

	response := protocolDecoder.Envelope.Payload
	reply := response.(protocol.ConnectResponsePacket)
	if reply.Established {
		defer localConn.Terminate(true)
		var wq waiter.Queue
		if localConn.IsTCP() {
			ep, iperr := localConn.GetTCP().Request.CreateEndpoint(&wq)
			if iperr != nil {
				slog.Debug("Packet handler encountered an error #4",
					slog.Any("error", iperr),
				)
				return
			}
			gonetConn := gonet.NewTCPConn(&wq, ep)
			relay.StartRelay(yamuxConnectionSession, gonetConn)
			ep.Abort() // I don't like this, but TIME_WAIT overflows within gvisor otherwise -- gotta investigate
		} else if localConn.IsUDP() {
			defer localConn.Terminate(false)
			ep, iperr := localConn.GetUDP().Request.CreateEndpoint(&wq)
			if iperr != nil {
				slog.Error("Packet handler encountered an error #5",
					slog.Any("error", iperr),
				)
				return
			}

			gonetConn := gonet.NewUDPConn(&wq, ep)
			relay.StartRelay(yamuxConnectionSession, gonetConn)
		}
	} else {
		localConn.Terminate(reply.Reset)
	}
}

// tryAddICMP adds a TunConn to the connection pool under the NetStack mutex.
// Extracting the locked section to a helper ensures defer fires per-iteration,
// not at goroutine exit, and prevents the mutex from leaking on panic.
func (ns *NetStack) tryAddICMP(tunConn TunConn) bool {
	ns.Lock()
	defer ns.Unlock()

	if ns.pool == nil || ns.pool.Closed() {
		return false
	}

	if err := ns.pool.Add(tunConn); err != nil {
		slog.Error("ICMP responder encountered an error", slog.Any("error", err))
		return false
	}

	return true
}

// handleICMP process incoming ICMP packets and, depending on the target host status, respond a ICMP ECHO Reply
// Please note that other ICMP messages are not yet supported.
func (ns *NetStack) handleICMP(localConn TunConn, multiplex *yamux.Session, localRoutes []route.Route) {
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
		defer yamuxConnectionSession.Close()

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
			ns.ProcessICMP(&pkt)

		}

	}
	// Ignore other ICMPs
}

// ProcessICMP send back a ICMP echo reply from after receiving a echo request.
// This code come mostly from pkg/tcpip/network/ipv4/icmp.go
func (ns *NetStack) ProcessICMP(pkt *stack.PacketBuffer) {
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
	// Go's select has no case priorities, so we use a nested select:
	// the outer non-blocking arm catches an already-closed pool before
	// competing with p.Pool; the inner blocking select handles a concurrent
	// Close() that fires while we are waiting for buffer space.
	select {
	case <-p.CloseChan:
	default:
		select {
		case p.Pool <- packet:
			return nil
		case <-p.CloseChan:
		}
	}
	return errors.New("pool is closed")
}

func (p *ConnPool) Close() error {
	p.Lock()
	defer p.Unlock()

	select {
	case <-p.CloseChan:
		return errors.New("pool is already closed")
	default:
		// Close only CloseChan; do not close Pool so that concurrent Add()
		// selects on Pool<-packet can't panic from sending to a closed channel.
		// Once CloseChan is closed, all Add()/Get() selects will pick that arm.
		close(p.CloseChan)
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
		},
		HandleLocal: false,
	})

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

	linkEP, _, err := tun.New(tunName)
	if err != nil {
		return nil, err
	}

	// Wrap the link endpoint so ICMP echo requests are intercepted before
	// reaching gVisor's IPv4 handler (which would auto-reply unconditionally).
	// The interceptor routes echo requests to the ConnPool so handleICMP can
	// probe the agent and reply only when the target host is actually alive.
	wrappedEP := &interceptingEndpoint{LinkEndpoint: linkEP, ns: ns}

	// Create a new NIC
	if err := ns.stack.CreateNIC(1, wrappedEP); err != nil {
		return nil, errors.New(err.String())
	}

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
