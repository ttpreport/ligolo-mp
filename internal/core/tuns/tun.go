package tuns

import (
	"fmt"
	"net"

	pb "github.com/ttpreport/ligolo-mp/protobuf"
	"github.com/vishvananda/netlink"
)

type Tun struct {
	Link       netlink.Link
	Routes     map[string]netlink.Route
	Name       string
	Alias      string
	IsLoopback bool
	Active     bool
}

func newTun(name string, alias string, isLoopback bool) (*Tun, error) {
	ret := &Tun{}
	la := netlink.NewLinkAttrs()
	la.Name = name

	ret.Name = name
	ret.Alias = alias
	ret.IsLoopback = isLoopback
	ret.Link = &netlink.Tuntap{
		LinkAttrs: la,
		Mode:      netlink.TUNTAP_MODE_TUN,
	}

	var err error
	if err = netlink.LinkAdd(ret.Link); err != nil {
		return nil, err
	}

	if err = netlink.LinkSetUp(ret.Link); err != nil {
		return nil, err
	}

	ret.Routes = make(map[string]netlink.Route)

	return ret, nil
}

func restoreTun(name string, alias string, isLoopback bool) (*Tun, error) {
	var err error
	ret := &Tun{}
	ret.Link, err = netlink.LinkByName(name)

	if err != nil {
		return nil, err
	}

	if err = netlink.LinkSetUp(ret.Link); err != nil {
		return nil, err
	}

	ret.Name = name
	ret.Alias = alias
	ret.IsLoopback = isLoopback

	routes, err := netlink.RouteList(ret.Link, netlink.FAMILY_ALL)

	if err != nil {
		return nil, err
	}

	ret.Routes = make(map[string]netlink.Route)
	for _, route := range routes {
		ret.Routes[route.Dst.String()] = route
	}

	return ret, nil
}

func (tun *Tun) String() string {
	return fmt.Sprintf("%s (%s)", tun.Alias, tun.Name)
}

func (tun *Tun) Proto() *pb.Tun {
	var Routes []*pb.Route
	for _, route := range tun.Routes {
		Routes = append(Routes, &pb.Route{Cidr: route.Dst.String()})
	}
	return &pb.Tun{
		Alias:      tun.Alias,
		Routes:     Routes,
		IsLoopback: tun.IsLoopback,
	}
}

func (tun *Tun) destroy() error {
	return netlink.LinkDel(tun.Link)
}

func (tun *Tun) addRoute(cidr string) error {
	var err error
	_, dst, err := net.ParseCIDR(cidr)

	if err != nil {
		return err
	}

	route := netlink.Route{
		LinkIndex: tun.Link.Attrs().Index,
		Dst:       dst,
	}

	if err := netlink.RouteAdd(&route); err != nil {
		return err
	}

	tun.Routes[cidr] = route

	return nil
}

func (tun *Tun) deleteRoute(cidr string) error {
	route, ok := tun.Routes[cidr]
	if !ok {
		return nil
	}

	if err := netlink.RouteDel(&route); err != nil {
		return err
	}

	delete(tun.Routes, cidr)

	return nil
}
