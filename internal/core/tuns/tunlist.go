package tuns

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"

	"github.com/ttpreport/ligolo-mp/internal/common/events"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
)

type Tuns struct {
	active map[string]*Tun
	mutex  *sync.RWMutex
	store  *storage.Store
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func New(storage *storage.Store) *Tuns {
	return &Tuns{
		active: make(map[string]*Tun),
		mutex:  &sync.RWMutex{},
		store:  storage,
	}
}

func (tuns *Tuns) Create(name string, isLoopback bool) (Tun, error) {
	tuns.mutex.Lock()
	defer tuns.mutex.Unlock()

	var err error

	if name == "" {
		suffix, err := randomHex(6)
		if err != nil {
			return Tun{}, err
		}

		name = fmt.Sprintf("lig%s", suffix)
	}

	slog.Info("Create tun", slog.Any("name", name))

	tun, err := newTun(name, name, isLoopback)
	if err != nil {
		slog.Error("Couldn't add tun",
			slog.Any("reason", err),
		)
		return Tun{}, nil
	}

	tuns.active[name] = tun

	if err := tuns.store.AddTun(name, name, isLoopback); err != nil {
		slog.Warn("Couldn't save tun to storage",
			slog.Any("reason", err),
		)
	}

	events.EventStream <- events.Event{Type: events.TunNew, Data: *tun}

	return *tun, nil
}

func (tuns *Tuns) Restore() error {
	tuns.mutex.Lock()
	defer tuns.mutex.Unlock()

	var err error
	cached_tuns, err := tuns.store.GetTuns()

	if err != nil {
		return err
	}

	for _, cached_tun := range cached_tuns {
		tun, err := restoreTun(cached_tun.Name, cached_tun.Alias, cached_tun.IsLoopback)

		if err != nil {
			continue
		}

		tuns.active[cached_tun.Alias] = tun

		events.EventStream <- events.Event{Type: events.TunNew, Data: *tun}
	}

	return err
}

func (tuns *Tuns) Destroy(alias string) error {
	tuns.mutex.Lock()
	defer tuns.mutex.Unlock()

	if tun, ok := tuns.active[alias]; ok {
		err := tun.destroy()
		if err != nil {
			return err
		}

		delete(tuns.active, alias)

		events.EventStream <- events.Event{Type: events.TunLost, Data: *tun}
	}

	if err := tuns.store.DelTun(alias); err != nil {
		slog.Warn("Couldn't remove tun from storage",
			slog.Any("reason", err),
		)
	}

	return nil
}

func (tuns *Tuns) Rename(oldAlias, newAlias string) error {
	tuns.mutex.Lock()
	defer tuns.mutex.Unlock()

	if tun, ok := tuns.active[oldAlias]; ok {
		if err := tuns.store.RenameTun(oldAlias, newAlias); err != nil {
			slog.Warn("Couldn't rename tun in storage",
				slog.Any("reason", err),
			)
			return err
		}

		tun.Alias = newAlias

		events.EventStream <- events.Event{Type: events.TunRenamed, Data: *tun}
	}

	return nil
}

func (tuns *Tuns) GetOne(alias string) *Tun {
	tuns.mutex.RLock()
	defer tuns.mutex.RUnlock()

	return tuns.active[alias]
}

// I know
func (tuns *Tuns) List() map[string]*Tun {
	return tuns.active
}

func (tuns *Tuns) Len() int {
	tuns.mutex.RLock()
	defer tuns.mutex.RUnlock()

	return len(tuns.active)
}

func (tuns *Tuns) AddRoute(alias string, cidr string) error {
	tuns.mutex.Lock()
	defer tuns.mutex.Unlock()

	var err error
	if err = tuns.active[alias].addRoute(cidr); err != nil {
		return err
	}

	events.EventStream <- events.Event{Type: events.RouteNew, Data: TunRouteEvent{
		tun:  *tuns.active[alias],
		cidr: cidr,
	}}

	return nil
}

func (tuns *Tuns) DeleteRoute(alias string, cidr string) error {
	tuns.mutex.Lock()
	defer tuns.mutex.Unlock()

	var err error
	if err = tuns.active[alias].deleteRoute(cidr); err != nil {
		return err
	}

	events.EventStream <- events.Event{Type: events.RouteLost, Data: TunRouteEvent{
		tun:  *tuns.active[alias],
		cidr: cidr,
	}}

	return nil
}

func (tuns *Tuns) RouteOverlaps(cidr string) *Tun {
	tuns.mutex.RLock()
	defer tuns.mutex.RUnlock()

	for _, tun := range tuns.active {
		for _, route := range tun.Routes {
			CIDRPrefix, err := netip.ParsePrefix(cidr)
			if err != nil {
				continue
			}

			RoutePrefix, err := netip.ParsePrefix(route.Dst.String())
			if err != nil {
				continue
			}

			if RoutePrefix.Overlaps(CIDRPrefix) {
				return tun
			}
		}
	}

	return nil
}
