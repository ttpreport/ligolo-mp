package agent

import (
	"net"

	"github.com/ttpreport/ligolo-mp/internal/core/relay"
)

// Listener is the base class implementing listener sockets for Ligolo
type Listener struct {
	ID       int32
	Network  string
	From     string
	To       string
	Listener net.Listener
}

// NewListener register a new listener
func NewListener(network string, from string, to string) (Listener, error) {
	lis, err := net.Listen(network, from)

	if err != nil {
		return Listener{}, err
	}
	return Listener{Network: network, From: from, To: to, Listener: lis}, nil
}

func (s *Listener) ListenAndRelay() error {
	for {
		lconn, err := s.Listener.Accept()
		if err != nil {
			return err
		}

		rconn, err := net.Dial(s.Network, s.To)
		if err != nil {
			return err
		}

		go relay.StartRelay(lconn, rconn)
	}
}

// Close request the main listener to exit
func (s *Listener) Close() error {
	return s.Listener.Close()
}
