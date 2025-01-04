package widgets

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type ServerWidget struct {
	*tview.TextView
	operator     *pb.Operator
	serverConfig *pb.Config
	fetchConfig  func()
}

func NewServerWidget() *ServerWidget {
	widget := &ServerWidget{
		TextView: tview.NewTextView(),
	}

	widget.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("server")))
	widget.SetBorder(true)
	widget.SetTextAlign(tview.AlignCenter)

	return widget
}

func (widget *ServerWidget) SetData(metadata *pb.GetMetadataResp) {
	widget.Clear()
	widget.operator = metadata.Operator
	widget.serverConfig = metadata.Config
	widget.Refresh()
}

func (widget *ServerWidget) Refresh() {
	if widget.operator != nil {
		access := "operator"
		if widget.operator.IsAdmin {
			access = "admin"
		}

		text := fmt.Sprintf("Operator: %s@%s (%s) | Agent server: %s", widget.operator.Name, widget.operator.Server, access, widget.serverConfig.AgentServer)
		widget.SetText(text)
	}
}
