package relay

import (
	"net"
)

type Redirector struct {
	ID       string
	Network  string
	From     string
	To       string
	Listener net.Listener
}

func NewLRedirector(id string, network string, from string, to string) (Redirector, error) {
	lis, err := net.Listen(network, from)

	if err != nil {
		return Redirector{}, err
	}
	return Redirector{ID: id, Network: network, From: from, To: to, Listener: lis}, nil
}

func (s *Redirector) ListenAndRelay() error {
	for {
		lconn, err := s.Listener.Accept()
		if err != nil {
			return err
		}

		rconn, err := net.Dial(s.Network, s.To)
		if err != nil {
			return err
		}

		go StartRelay(lconn, rconn)
	}
}

func (s *Redirector) Close() error {
	return s.Listener.Close()
}
