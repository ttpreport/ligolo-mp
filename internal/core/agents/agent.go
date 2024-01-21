package agents

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/common/config"
	"github.com/ttpreport/ligolo-mp/internal/common/events"
	"github.com/ttpreport/ligolo-mp/internal/core/listeners"
	"github.com/ttpreport/ligolo-mp/internal/core/protocol"
	"github.com/ttpreport/ligolo-mp/internal/core/proxy/netstack"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
	"github.com/ttpreport/ligolo-mp/internal/core/tuns"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type Agent struct {
	Alias     string
	Name      string
	Hostname  string
	Network   []protocol.NetInterface
	Tun       *tuns.Tun
	Session   *yamux.Session
	CloseChan chan bool
	Listeners *listeners.Listeners
	config    *config.Config
}

func (agent *Agent) String() string {
	var session string
	if agent.Session != nil {
		session = agent.Session.RemoteAddr().String()
	} else {
		session = "dangling"
	}
	return fmt.Sprintf("%s - %s (%s)", agent.Alias, agent.Name, session)
}

func (agent *Agent) Proto() *pb.Agent {
	var Ips []string
	for _, ifaceInfo := range agent.Network {
		Ips = append(Ips, ifaceInfo.Addresses...)
	}

	var agentTun *pb.Tun
	if agent.Tun != nil {
		agentTun = agent.Tun.Proto()
	}

	return &pb.Agent{
		Alias:    agent.Alias,
		Hostname: agent.Hostname,
		Tun:      agentTun,
		IPs:      Ips,
	}
}

func newAgent(config *config.Config, alias string, session *yamux.Session, storage *storage.Store) (*Agent, error) {
	yamuxConnectionSession, err := session.Open()
	if err != nil {
		return &Agent{}, err
	}

	protocolEncoder := protocol.NewEncoder(yamuxConnectionSession)
	protocolDecoder := protocol.NewDecoder(yamuxConnectionSession)

	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageInfoRequest,
		Payload: protocol.InfoRequestPacket{},
	}); err != nil {
		return &Agent{}, err
	}

	if err := protocolDecoder.Decode(); err != nil {
		return &Agent{}, err
	}

	response := protocolDecoder.Envelope.Payload
	reply := response.(protocol.InfoReplyPacket)

	agent := &Agent{
		Alias:     alias,
		Name:      reply.Name,
		Hostname:  reply.Hostname,
		Network:   reply.Interfaces,
		Session:   session,
		CloseChan: make(chan bool),
		Listeners: listeners.New(storage),
		config:    config,
	}

	for _, listener := range reply.Listeners {
		agent.RestoreListener(listener.ID, listener.Network, listener.From, listener.To)
	}

	return agent, nil
}

func (agent *Agent) NewListener(network string, listener_addr string, redirect_addr string) (listeners.Listener, error) {
	new_listener, err := agent.Listeners.Create(network, listener_addr, redirect_addr, agent.Session)

	if err != nil {
		return listeners.Listener{}, err
	}

	return new_listener, nil
}

func (agent *Agent) DeleteListener(alias string) error {
	if err := agent.Listeners.Destroy(alias); err != nil {
		return err
	}

	return nil
}

func (agent *Agent) RestoreListener(listenerId int32, network string, listener_addr string, redirect_addr string) (listeners.Listener, error) {
	new_listener, err := agent.Listeners.Restore(listenerId, network, listener_addr, redirect_addr, agent.Session)

	if err != nil {
		return listeners.Listener{}, err
	}

	return new_listener, nil
}

func (agent *Agent) startRelay(tun *tuns.Tun) {
	tun.Active = true

	// Create a new, empty, connpool to store connections/packets
	connPool := netstack.NewConnPool(agent.config.MaxConnectionHandler)

	stackSettings := netstack.StackSettings{
		MaxInflight: agent.config.MaxInFlight,
		TunName:     tun.Name,
	}
	nstack, err := netstack.NewStack(stackSettings, &connPool)

	// Cleanup pool if channel is closed
	defer func() {
		connPool.Close()
		nstack.Destroy()
		agent.Tun = nil
		tun.Active = false
	}()

	if err != nil {
		return
	}

	agent.Tun = tun

	events.EventStream <- events.Event{Type: events.RelayNew, Data: *agent}

	for {
		select {
		case <-agent.CloseChan: // User stopped
			err := errors.New("closing relay due to user interrupt")
			events.EventStream <- events.Event{Type: events.RelayLost, Data: *agent, Error: err}
			return
		case <-agent.Session.CloseChan(): // Agent closed
			err := errors.New("lost connection with agent")
			events.EventStream <- events.Event{Type: events.RelayLost, Data: *agent, Error: err}
			return
		case <-connPool.CloseChan: // pool closed, we can't process packets!
			err := errors.New("connection pool closed")
			events.EventStream <- events.Event{Type: events.RelayLost, Data: *agent, Error: err}
			return
		case relayPacket := <-connPool.Pool: // Process connections/packets
			go netstack.HandlePacket(nstack.GetStack(), relayPacket, agent.Session, tun)
		}
	}
}

func (agent *Agent) stopRelay() {
	agent.CloseChan <- true
}

func (agent *Agent) Hash() string {
	hasher := sha1.New()
	for _, ifaceInfo := range agent.Network {
		hasher.Write([]byte(ifaceInfo.HardwareAddr))
		hasher.Write([]byte(ifaceInfo.Name))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
