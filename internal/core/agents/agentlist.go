package agents

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/internal/common/config"
	"github.com/ttpreport/ligolo-mp/internal/common/events"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
	"github.com/ttpreport/ligolo-mp/internal/core/tuns"
)

type Agents struct {
	active      map[string]*Agent
	mutex       *sync.RWMutex
	store       *storage.Store
	EventStream chan events.Event
}

func New(storage *storage.Store) *Agents {
	return &Agents{
		active: make(map[string]*Agent),
		mutex:  &sync.RWMutex{},
		store:  storage,
	}
}

func (agents *Agents) Create(config *config.Config, session *yamux.Session) (*Agent, error) {
	agents.mutex.Lock()
	defer agents.mutex.Unlock()

	var err error

	var alias string
	for i := 0; i < 6; i++ {
		alias = namesgenerator.GetRandomName(i)

		if _, ok := agents.active[alias]; !ok {
			break
		}
	}

	new_agent, err := newAgent(config, alias, session, agents.store)

	if err != nil {
		slog.Error("Couldn't create agent",
			slog.Any("reason", err),
		)
		return nil, err
	}

	events.EventStream <- events.Event{Type: events.AgentNew, Data: *new_agent}
	agents.active[alias] = new_agent

	return new_agent, nil
}

func (agents *Agents) Rename(oldAlias, newAlias string) error {
	agents.mutex.Lock()
	defer agents.mutex.Unlock()

	agent := agents.active[oldAlias]
	if agent == nil {
		return errors.New("agent not found")
	}

	if _, ok := agents.active[newAlias]; ok {
		return errors.New("new agent alias already exists")
	}

	events.EventStream <- events.Event{Type: events.AgentRenamed, Data: *agent}

	agents.active[newAlias] = agent
	delete(agents.active, oldAlias)

	agent.Alias = newAlias

	return nil
}

func (agents *Agents) Destroy(alias string) {
	agents.mutex.Lock()
	defer agents.mutex.Unlock()

	agent := agents.active[alias]
	for _, listener := range agent.Listeners.List() {
		listener.Destroy()
	}

	delete(agents.active, alias)
}

// I know
func (agents *Agents) List() map[string]*Agent {
	return agents.active
}

func (agents *Agents) Len() int {
	agents.mutex.RLock()
	defer agents.mutex.RUnlock()

	return len(agents.active)
}

func (agents *Agents) NewListener(alias string, network string, from string, to string) error {
	agents.mutex.Lock()
	defer agents.mutex.Unlock()

	agent := agents.active[alias]
	if agent == nil {
		return errors.New("agent not found")
	}

	_, err := agent.NewListener(network, from, to)

	return err
}

func (agents *Agents) DeleteListener(agentAlias string, listenerAlias string) error {
	agents.mutex.Lock()
	defer agents.mutex.Unlock()

	agent := agents.active[agentAlias]
	if agent == nil {
		return errors.New("agent not found")
	}

	return agent.DeleteListener(listenerAlias)
}

func (agents *Agents) StartRelay(alias string, tun *tuns.Tun) error {
	agents.mutex.RLock()
	defer agents.mutex.RUnlock()

	agent := agents.active[alias]
	if agent == nil {
		return errors.New("agent not found")
	}

	if agent.Tun != nil {
		return errors.New("this agent is already running a relay")
	}

	if tun.Active {
		return errors.New("this tun is already running a relay")
	}

	if err := agents.store.AddRelay(tun.Alias, agent.Hash()); err != nil {
		return err
	}

	go agent.startRelay(tun)

	return nil
}

func (agents *Agents) RestoreRelay(alias string, tuns *tuns.Tuns) error {
	agents.mutex.Lock()
	defer agents.mutex.Unlock()

	agent := agents.active[alias]
	if agent == nil {
		return errors.New("agent not found")
	}

	cached_agent_tun, err := agents.store.GetAgentTun(agent.Hash())

	if err != nil {
		return err
	}

	if cached_agent_tun != nil {
		tun := tuns.GetOne(cached_agent_tun.TunAlias)
		go agent.startRelay(tun)
	}

	return nil
}

func (agents *Agents) StopRelay(alias string) error {
	agents.mutex.Lock()
	defer agents.mutex.Unlock()

	agent := agents.active[alias]
	if agent == nil {
		return errors.New("agent not found")
	}

	if agent.Tun == nil {
		return errors.New("no active relays")
	}

	agent.stopRelay()

	if err := agents.store.DelRelay(agent.Hash()); err != nil {
		return err
	}

	return nil
}
