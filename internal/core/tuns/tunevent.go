package tuns

import (
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type TunRouteEvent struct {
	tun  Tun
	cidr string
}

func (event *TunRouteEvent) Proto() *pb.TunRoute {
	return &pb.TunRoute{
		Tun:   event.tun.Proto(),
		Route: &pb.Route{Cidr: event.cidr},
	}
}
