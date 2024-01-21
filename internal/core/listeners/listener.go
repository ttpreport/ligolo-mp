package listeners

import (
	"errors"
	"fmt"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/common/events"
	"github.com/ttpreport/ligolo-mp/internal/core/protocol"

	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type Listener struct {
	Id           int32
	Alias        string
	Network      string
	ListenerAddr string
	RedirectAddr string
	Session      *yamux.Session
	Errors       chan error
}

func newListener(alias string, network string, listener_addr string, redirect_addr string, session *yamux.Session) (Listener, error) {
	// Open a new Yamux Session
	yamuxConnectionSession, err := session.Open()
	if err != nil {
		return Listener{}, err
	}
	protocolEncoder := protocol.NewEncoder(yamuxConnectionSession)
	protocolDecoder := protocol.NewDecoder(yamuxConnectionSession)

	// Request to open a new port on the agent
	listenerPacket := protocol.ListenerRequestPacket{Network: network, From: listener_addr, To: redirect_addr}
	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageListenerRequest,
		Payload: listenerPacket,
	}); err != nil {
		return Listener{}, err
	}

	// Get response from agent
	if err := protocolDecoder.Decode(); err != nil {
		return Listener{}, err
	}
	listenerResponse := protocolDecoder.Envelope.Payload.(protocol.ListenerResponsePacket)
	if listenerResponse.Err {
		return Listener{}, errors.New(listenerResponse.ErrString)
	}

	yamuxConnectionSession.Close()

	listener := Listener{
		Id:           listenerResponse.ListenerID,
		Alias:        alias,
		Network:      network,
		ListenerAddr: listener_addr,
		RedirectAddr: redirect_addr,
		Session:      session,
	}

	events.EventStream <- events.Event{Type: events.ListenerNew, Data: listener}

	return listener, nil
}

func restoreListener(alias string, listenerID int32, network string, listener_addr string, redirect_addr string, session *yamux.Session) (Listener, error) {
	listener := Listener{
		Id:           listenerID,
		Alias:        alias,
		Network:      network,
		ListenerAddr: listener_addr,
		RedirectAddr: redirect_addr,
		Session:      session,
	}

	events.EventStream <- events.Event{Type: events.ListenerNew, Data: listener}

	return listener, nil
}

func (listener *Listener) String() string {
	return fmt.Sprintf("%s (%s -> %s)", listener.Alias, listener.ListenerAddr, listener.RedirectAddr)
}

func (listener *Listener) Proto() *pb.Listener {
	return &pb.Listener{
		Alias: listener.Alias,
		From:  listener.ListenerAddr,
		To:    listener.RedirectAddr,
	}
}

func (listener *Listener) Destroy() error {
	events.EventStream <- events.Event{Type: events.ListenerLost, Data: *listener}

	yamuxConnectionSession, err := listener.Session.Open()

	if err != nil {
		return err
	}

	protocolEncoder := protocol.NewEncoder(yamuxConnectionSession)
	protocolDecoder := protocol.NewDecoder(yamuxConnectionSession)

	// Send close request
	closeRequest := protocol.ListenerCloseRequestPacket{ListenerID: listener.Id}
	if err := protocolEncoder.Encode(protocol.Envelope{
		Type:    protocol.MessageListenerCloseRequest,
		Payload: closeRequest,
	}); err != nil {
		return err
	}

	// Process close response
	if err := protocolDecoder.Decode(); err != nil {
		return err

	}
	response := protocolDecoder.Envelope.Payload

	if err := response.(protocol.ListenerCloseResponsePacket).Err; err {
		return errors.New(response.(protocol.ListenerCloseResponsePacket).ErrString)
	}

	if err := yamuxConnectionSession.Close(); err != nil {
		return err
	}

	return nil
}
