package cli

import (
	"context"
	"time"

	"github.com/ttpreport/ligolo-mp/internal/events"
	"github.com/ttpreport/ligolo-mp/internal/operator"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

func handleEvents(oper *operator.Operator) error {
	for {
		var eventStream pb.Ligolo_JoinClient
		var err error

		for {
			printOk("Connecting...")

			eventStream, err = oper.Client().Join(context.Background(), &pb.Empty{})
			if err == nil {
				break
			} else {
				printErr("Could not connect, retrying...")
				time.Sleep(5 * time.Second)
			}
		}

		for {
			event, err := eventStream.Recv()

			if err != nil {
				printErr("Lost connection to the server")
				time.Sleep(5 * time.Second)
				break
			}

			switch event.Type {
			case int32(events.OK):
				printOk(event.Data)
			case int32(events.ERROR):
				printErr(event.Data)
			}
		}
	}
}
