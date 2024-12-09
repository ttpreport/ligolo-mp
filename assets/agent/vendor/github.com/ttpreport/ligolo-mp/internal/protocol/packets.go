package protocol

import (
	"net"

	"github.com/ttpreport/ligolo-mp/internal/relay"
)

// Envelope is the structure used when Encoding/Decode ligolo packets
type Envelope struct {
	Type    uint8
	Size    int32
	Payload interface{}
}

const (
	MessageInfoRequest = uint8(iota)
	MessageInfoReply
	MessageConnectRequest
	MessageConnectResponse
	MessageHostPingRequest
	MessageHostPingResponse
	MessageRedirectorRequest
	MessageRedirectorResponse
	MessageRedirectorBindRequest
	MessageRedirectorBindResponse
	MessageRedirectorCloseRequest
	MessageRedirectorCloseResponse
	MessageDisconnectRequest
	MessageDisconnectResponse
)

const (
	TransportTCP = uint8(iota)
	TransportUDP
)

const (
	Networkv4 = uint8(iota)
	Networkv6
)

type InfoRequestPacket struct {
}

type InfoReplyPacket struct {
	Name        string
	Hostname    string
	Interfaces  []NetInterface
	Redirectors []RedirectorInterface
}

type RedirectorRequestPacket struct {
	ID      string
	Network string
	From    string
	To      string
}

type RedirectorResponsePacket struct {
	ID        string
	Err       bool
	ErrString string
}

type RedirectorCloseRequestPacket struct {
	ID string
}

type RedirectorCloseResponsePacket struct {
	ErrString string
	Err       bool
}

// NetInterface is the structure containing the agent network informations
type NetInterface struct {
	Index        int              // positive integer that starts at one, zero is never used
	MTU          int              // maximum transmission unit
	Name         string           // e.g., "en0", "lo0", "eth0.100"
	HardwareAddr net.HardwareAddr // IEEE MAC-48, EUI-48 and EUI-64 form
	Flags        net.Flags        // e.g., FlagUp, FlagLoopback, FlagMulticast
	Addresses    []string
}

// NewNetInterfaces converts a net.Interface slice to a NetInterface slice that can be transmitted over Gob
func NewNetInterfaces(netif []net.Interface) (out []NetInterface) {
	// the net.Interface struct doesn't contains the IP Address, we need a new struct that store IPs
	for _, iface := range netif {
		var addrs []string
		addresses, err := iface.Addrs()
		if err != nil {
			addresses = []net.Addr{}
		}
		for _, addrStr := range addresses {
			addrs = append(addrs, addrStr.String())
		}
		out = append(out, NetInterface{
			Index:        iface.Index,
			MTU:          iface.MTU,
			Name:         iface.Name,
			HardwareAddr: iface.HardwareAddr,
			Flags:        iface.Flags,
			Addresses:    addrs,
		})
	}
	return
}

func NewRedirectorInterface(redirectors map[string]relay.Redirector) (out []RedirectorInterface) {
	for _, redirector := range redirectors {
		out = append(out, RedirectorInterface{
			ID:      redirector.ID,
			Network: redirector.Network,
			From:    redirector.From,
			To:      redirector.To,
		})
	}
	return
}

type RedirectorInterface struct {
	ID      string
	Network string
	From    string
	To      string
}

// ConnectRequestPacket is sent by the proxy to request a new TCP/UDP connection
type ConnectRequestPacket struct {
	Net       uint8
	Transport uint8
	Address   string
	Port      uint16
}

// ConnectResponsePacket is the response to the ConnectRequestPacket and indicate if the connection can be established, and if a RST packet need to be sent
type ConnectResponsePacket struct {
	Established bool
	Reset       bool
}

// DisconnectRequestPacket is send to terminate agent
type DisconnectRequestPacket struct{}

// DisconnectResponsePacket is the response to the DisconnectRequestPacket and indicate if the connection is closed
type DisconnectResponsePacket struct{}

// HostPingRequestPacket is used when a ICMP packet is received on the proxy server. It is used to request a ping request to the agent
type HostPingRequestPacket struct {
	Address string
}

// HostPingResponsePacket is sent by the agent to indicate the requested host status
type HostPingResponsePacket struct {
	Alive bool
}
