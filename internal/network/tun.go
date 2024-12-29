package network

import (
	"log/slog"
	"net"

	"github.com/hashicorp/yamux"
	"github.com/ttpreport/ligolo-mp/pkg/memstore"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
	"github.com/vishvananda/netlink"
)

type Tun struct {
	ID       int
	Name     string
	Active   bool
	Routes   *memstore.Syncmap[string, *Route]
	netstack *NetStack `json:"-"`
}

type Route struct {
	Cidr       *net.IPNet
	IsLoopback bool
}

func NewTun() (*Tun, error) {
	slog.Debug("creating new tun")
	ret := &Tun{
		Routes: memstore.NewSyncmap[string, *Route](),
	}

	return ret, nil
}

func (t *Tun) Start(multiplex *yamux.Session, maxConnections int, maxInFlight int) error {
	if t.Active {
		return nil
	}

	slog.Debug("creating tun")
	la := netlink.NewLinkAttrs()

	link := &netlink.Tuntap{
		LinkAttrs: la,
		Mode:      netlink.TUNTAP_MODE_TUN,
	}
	if err := netlink.LinkAdd(link); err != nil {
		return err
	}
	slog.Debug("tun link created", slog.Any("link", link))

	t.ID = link.Index
	t.Name = link.Name

	if err := netlink.LinkSetUp(link); err != nil {
		return err
	}
	slog.Debug("interface set up")

	ns, err := NewNetstack(maxConnections, maxInFlight, t.Name)
	if err != nil {
		slog.Error("could not create netstack")
		return err
	}
	slog.Debug("netstack created", slog.Any("netstack", ns))

	t.netstack = ns

	go func() {
		for {
			select {
			case <-t.netstack.ClosePool(): // pool closed, we can't process packets!
				slog.Debug("connection pool closed")
				return
			case relayPacket := <-t.netstack.GetTunConn(): // Process connections/packets
				localRoutes := t.GetLocalRoutes() // a bit dirty, but allows granular localhost routing
				go t.netstack.HandlePacket(relayPacket, multiplex, localRoutes)
			}
		}
	}()

	t.Active = true

	slog.Debug("tun activated")

	if err := t.ApplyRoutes(); err != nil {
		slog.Error("could not apply routes")
	}

	return nil
}

func (t *Tun) Stop() {
	slog.Debug("removing tun", slog.Any("tun", t))

	link, err := netlink.LinkByIndex(t.ID)
	if err != nil {
		slog.Warn("link not found", slog.Any("err", err), slog.Any("id", t.ID))
	} else {
		if err := netlink.LinkDel(link); err != nil {
			slog.Error("could not delete link", slog.Any("error", err))
		}
	}

	slog.Debug("tun removed", slog.Any("tun", t))

	if t.netstack == nil {
		slog.Warn("netstack not found")
	} else {
		if err := t.netstack.Destroy(); err != nil {
			slog.Error("could not destroy netstack", slog.Any("err", err), slog.Any("netstack", t.netstack))
		}
	}

	t.Active = false
}

func (t *Tun) ApplyRoutes() error {
	if t.Active {
		slog.Debug("applying routes")

		if err := t.removeAllRoutes(); err != nil {
			slog.Warn("could not remove current routes", slog.Any("err", err))
		}

		for _, route := range t.Routes.All() {
			route := &netlink.Route{
				LinkIndex: t.ID,
				Dst:       route.Cidr,
			}
			if err := netlink.RouteAdd(route); err != nil {
				slog.Warn("could not add route to the system", slog.Any("err", err), slog.Any("route", route))
			}
		}
	}

	return nil
}

func (t *Tun) removeAllRoutes() error {
	link, err := netlink.LinkByIndex(t.ID)
	if err != nil {
		slog.Error("could not find link", slog.Any("link_id", t.ID))
		return err
	}

	currentRoutes, err := netlink.RouteList(link, netlink.FAMILY_ALL)
	if err != nil {
		slog.Warn("could not get current system routes", slog.Any("link", link))
	}

	for _, route := range currentRoutes {
		if err := netlink.RouteDel(&route); err != nil {
			slog.Warn("could not delete route", slog.Any("route", route))
		}
	}

	return nil
}

func (t *Tun) NewRoute(cidr string, isLoopback bool) error {
	slog.Debug("adding route to tun", slog.Any("route", cidr))

	_, dst, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	t.Routes.Set(dst.String(), &Route{
		Cidr:       dst,
		IsLoopback: isLoopback,
	})

	slog.Debug("route added to tun")

	return nil
}

func (t *Tun) RemoveRoute(cidr string) error {
	slog.Debug("removing route from tun", slog.Any("route", cidr))

	_, dst, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	t.Routes.Delete(dst.String())

	if err := t.ApplyRoutes(); err != nil {
		slog.Error("could not apply routes", slog.Any("tun", t), slog.Any("routes", t.Routes.All()))
		return err
	}

	slog.Debug("route removed", slog.Any("routes", cidr))

	return nil
}

func (t *Tun) GetName() (string, error) {
	link, err := netlink.LinkByIndex(t.ID)
	if err != nil {
		return "", err
	}
	return link.Attrs().Name, nil
}

func (t *Tun) GetRoutes() []Route {
	var result []Route
	for _, route := range t.Routes.All() {
		result = append(result, *route)
	}

	return result
}

func (t *Tun) GetLocalRoutes() []Route {
	var routes []Route
	for _, route := range t.Routes.All() {
		if route.IsLoopback {
			routes = append(routes, *route)
		}
	}

	return routes
}

func (t *Tun) Proto() *pb.Tun {
	var Routes []*pb.Route
	for _, route := range t.Routes.All() {
		Routes = append(Routes, &pb.Route{
			Cidr:       route.Cidr.String(),
			IsLoopback: route.IsLoopback,
		})
	}
	return &pb.Tun{
		Name:   t.Name,
		Routes: Routes,
	}
}
