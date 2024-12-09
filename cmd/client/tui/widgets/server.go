package widgets

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/internal/operator"
)

type ServerWidget struct {
	*tview.TextView
	data *operator.Operator
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

func (widget *ServerWidget) SetData(data *operator.Operator) {
	widget.Clear()
	widget.data = data
	widget.Refresh()
}

func (widget *ServerWidget) Refresh() {
	if widget.data != nil {
		access := "regular"
		if widget.data.IsAdmin {
			access = "administrative"
		}

		text := fmt.Sprintf("You are connected to %s as %s who is %s user", widget.data.Server, widget.data.Name, access)
		widget.SetText(text)
	}
}
