package rpc

import (
	"log/slog"

	"github.com/ttpreport/ligolo-mp/internal/common/events"
	"github.com/ttpreport/ligolo-mp/internal/core/agents"
	"github.com/ttpreport/ligolo-mp/internal/core/listeners"
	"github.com/ttpreport/ligolo-mp/internal/core/tuns"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
	"google.golang.org/protobuf/proto"
)

func (s *ligoloServer) HandleEvents() {
	for {
		event := <-events.EventStream
		var pbData []byte
		var err error

		switch event.Type {
		case events.OperatorNew:
			slog.Info("Operator joined",
				slog.Any("operator", event.Data),
			)

			pbData, err = proto.Marshal(&pb.Operator{
				Name: event.Data.(string),
			})
		case events.OperatorLost:
			slog.Info("Operator disconnected",
				slog.Any("operator", event.Data),
			)

			pbData, err = proto.Marshal(&pb.Operator{
				Name: event.Data.(string),
			})
		case events.AgentNew:
			agent := event.Data.(agents.Agent)
			slog.Info("Agent joined",
				slog.Any("agent", agent),
			)

			pbData, err = proto.Marshal(agent.Proto())
		case events.AgentRenamed:
			agent := event.Data.(agents.Agent)
			slog.Info("Agent renamed",
				slog.Any("agent", agent),
			)

			pbData, err = proto.Marshal(agent.Proto())
		case events.AgentLost:
			agent := event.Data.(agents.Agent)
			slog.Warn("Agent disconnected",
				slog.Any("agent", agent),
				slog.Any("reason", event.Error),
			)

			pbObject, err := proto.Marshal(agent.Proto())
			if err == nil {
				pbData, err = proto.Marshal(&pb.Error{
					Object: pbObject,
					Reason: event.Error.Error(),
				})
			}
		case events.AgentErr:
			slog.Error("Agent encountered an error",
				slog.Any("agent", event.Data),
				slog.Any("reason", event.Error),
			)

			if event.Data != nil {
				agent := event.Data.(agents.Agent)

				pbAgent, err := proto.Marshal(agent.Proto())
				if err == nil {
					pbData, err = proto.Marshal(&pb.Error{
						Object: pbAgent,
						Reason: event.Error.Error(),
					})
				}
			} else {
				pbData, err = proto.Marshal(&pb.Error{
					Object: nil,
					Reason: event.Error.Error(),
				})
			}
		case events.TunNew:
			tun := event.Data.(tuns.Tun)
			slog.Info("New TUN created",
				slog.Any("tun", event.Data),
			)

			pbData, err = proto.Marshal(tun.Proto())
		case events.TunLost:
			tun := event.Data.(tuns.Tun)
			slog.Info("TUN lost",
				slog.Any("tun", event.Data),
			)

			pbData, err = proto.Marshal(tun.Proto())
		case events.TunRenamed:
			tun := event.Data.(tuns.Tun)
			slog.Info("TUN renamed",
				slog.Any("tun", event.Data),
			)

			pbData, err = proto.Marshal(tun.Proto())
		case events.RouteNew:
			data := event.Data.(tuns.TunRouteEvent)
			slog.Info("New route created",
				slog.Any("data", event.Data),
			)

			pbData, err = proto.Marshal(data.Proto())
		case events.RouteLost:
			data := event.Data.(tuns.TunRouteEvent)
			slog.Info("Route lost",
				slog.Any("data", event.Data),
			)

			pbData, err = proto.Marshal(data.Proto())
		case events.RelayNew:
			agent := event.Data.(agents.Agent)

			slog.Info("Relay established",
				slog.Any("agent", event.Data),
				slog.Any("tun", agent.Tun),
			)

			pbData, err = proto.Marshal(agent.Proto())
		case events.RelayLost:
			agent := event.Data.(agents.Agent)
			slog.Warn("Relay closed",
				slog.Any("agent", event.Data),
				slog.Any("tun", agent.Tun),
			)

			pbData, err = proto.Marshal(agent.Proto())
		case events.RelayErr:
			slog.Warn("Relay encountered an error",
				slog.Any("agent", event.Data),
				slog.Any("reason", event.Error),
			)

			var pbAgent []byte
			if event.Data != nil {
				agent := event.Data.(agents.Agent)
				pbAgent, err = proto.Marshal(agent.Proto())
				if err == nil {
					pbData, err = proto.Marshal(&pb.Error{
						Object: pbAgent,
						Reason: event.Error.Error(),
					})
				}
			} else {
				pbData, err = proto.Marshal(&pb.Error{
					Object: nil,
					Reason: event.Error.Error(),
				})
			}
		case events.ListenerNew:
			listener := event.Data.(listeners.Listener)

			slog.Info("Listener started",
				slog.Any("listener", event.Data),
			)

			pbData, err = proto.Marshal(listener.Proto())
		case events.ListenerLost:
			listener := event.Data.(listeners.Listener)

			slog.Info("Listener removed",
				slog.Any("listener", event.Data),
			)

			pbData, err = proto.Marshal(listener.Proto())
		case events.ListenerErr:
			slog.Warn("Listener encountered an error",
				slog.Any("listener", event.Data),
			)

			if event.Data != nil {
				listener := event.Data.(listeners.Listener)
				pbListener, err := proto.Marshal(listener.Proto())
				if err == nil {
					pbData, err = proto.Marshal(&pb.Error{
						Object: pbListener,
						Reason: event.Error.Error(),
					})
				}
			} else {
				pbData, err = proto.Marshal(&pb.Error{
					Object: nil,
					Reason: event.Error.Error(),
				})
			}
		default:
		}

		if err != nil {
			slog.Debug("Error marshalling event data",
				slog.Any("reason", err),
			)
		} else {
			pbEvent := &pb.Event{
				Type: event.Type,
				Data: pbData,
			}

			for _, stream := range s.operators {
				slog.Debug("trying to send event to operator")
				if err = stream.Send(pbEvent); err != nil {
					slog.Error("Sending event to operator failed", slog.Any("reason", err))
				}
			}
		}
	}
}
