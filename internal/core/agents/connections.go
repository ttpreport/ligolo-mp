package agents

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/common/config"
	"github.com/ttpreport/ligolo-mp/internal/common/events"
	"github.com/ttpreport/ligolo-mp/internal/core/proxy"
	"github.com/ttpreport/ligolo-mp/internal/core/tuns"
)

func (agents *Agents) WaitForConnections(config *config.Config, proxyController *proxy.Controller, tuns *tuns.Tuns) {
	for {
		var err error
		remoteConn := <-proxyController.Connection

		yamuxConn, err := yamux.Client(remoteConn, nil)
		if err != nil {
			events.EventStream <- events.Event{Type: events.AgentErr, Error: err}
		}
		fmt.Printf("Local: %s :: Remote: %s", yamuxConn.LocalAddr().String(), yamuxConn.RemoteAddr().String())
		agent, err := agents.Create(config, yamuxConn)
		if err != nil {
			events.EventStream <- events.Event{Type: events.AgentErr, Error: err}
			continue
		}

		err = agents.RestoreRelay(agent.Alias, tuns)
		if err != nil {
			events.EventStream <- events.Event{Type: events.AgentErr, Error: err}
		}

		go func() {
			for {
				if agent.Session.IsClosed() {
					err = errors.New("session closed")
					events.EventStream <- events.Event{Type: events.AgentLost, Error: err, Data: *agent}
					agents.Destroy(agent.Alias)
					return
				}

				time.Sleep(time.Second * 1)
			}
		}()
	}

}
