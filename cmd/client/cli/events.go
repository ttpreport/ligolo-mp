package cli

import (
	"io"
	"strings"

	"github.com/ttpreport/ligolo-mp/internal/common/events"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
	"google.golang.org/protobuf/proto"
)

func handleEvents(eventStream pb.Ligolo_JoinClient) {
	for {
		event, err := eventStream.Recv()
		if err == io.EOF {
			return
		}

		if err != nil {
			panic(err)
		}

		switch event.Type {
		case events.OperatorNew:
			data := &pb.Operator{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printOk("Operator %s joined", data.Name)
			}
		case events.OperatorLost:
			data := &pb.Operator{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printWarn("Operator %s disconnected", data.Name)
			}
		case events.AgentNew:
			data := &pb.Agent{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printOk("Agent %s@%s joined", data.Alias, data.Hostname)
			}
		case events.AgentLost:
			data := &pb.Error{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				agent := &pb.Agent{}
				err = proto.Unmarshal(data.Object, agent)
				if err == nil {
					printWarn("Agent %s@%s disconnected: %s", agent.Alias, strings.Join(agent.IPs, ","), data.Reason)
				}
			}
		case events.AgentErr:
			data := &pb.Error{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				agent := &pb.Agent{}
				err = proto.Unmarshal(data.Object, agent)
				if err == nil {
					printErr("Agent %s@%s encountered an error: %s", agent.Alias, strings.Join(agent.IPs, ","), data.Reason)
				}
			}
		case events.TunNew:
			data := &pb.Tun{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printOk("New TUN created: %s", data.Alias)
			}
		case events.TunLost:
			data := &pb.Tun{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printWarn("TUN lost: %s", data.Alias)
			}
		case events.RouteNew:
			data := &pb.TunRoute{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printOk("Route %s@%s created", data.Route.Cidr, data.Tun.Alias)
			}
		case events.RouteLost:
			data := &pb.TunRoute{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printWarn("Route %s@%s lost", data.Route.Cidr, data.Tun.Alias)
			}
		case events.RelayNew:
			data := &pb.Agent{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printOk("Established a relay with agent %s@%s", data.Alias, strings.Join(data.IPs, ","))
			}
		case events.RelayLost:
			data := &pb.Agent{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printWarn("Relay with agent %s@%s lost", data.Alias, strings.Join(data.IPs, ","))
			}
		case events.RelayErr:
			data := &pb.Error{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				agent := &pb.Agent{}
				err = proto.Unmarshal(data.Object, agent)
				if err == nil {
					printErr("Relay %s@%s encountered an error: %s", agent.Alias, strings.Join(agent.IPs, ","), data.Reason)
				}
			}
		case events.ListenerNew:
			data := &pb.Listener{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printOk("Listener (%s) from %s to %s started", data.Alias, data.From, data.To)
			}
		case events.ListenerLost:
			data := &pb.Listener{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				printWarn("Listener (%s) from %s to %s stopped", data.Alias, data.From, data.To)
			}
		case events.ListenerErr:
			data := &pb.Error{}
			err = proto.Unmarshal(event.Data, data)
			if err == nil {
				listener := &pb.Listener{}
				err = proto.Unmarshal(data.Object, listener)
				if err == nil {
					printErr("Listener (%s) from %s to %s encountered an error: %s", listener.Alias, listener.From, listener.To, data.Reason)
				}
			}
		default:
		}

		if err != nil {
			App.PrintError(err)
		}
	}
}
