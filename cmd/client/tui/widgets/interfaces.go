package widgets

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type InterfacesWidget struct {
	*tview.Table
	data []*pb.Interface
}

func NewInterfacesWidget() *InterfacesWidget {
	widget := &InterfacesWidget{
		Table: tview.NewTable(),
	}

	widget.SetSelectable(false, false)
	widget.SetBackgroundColor(style.BgColor)
	widget.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("interfaces")))
	widget.SetBorderColor(style.BorderColor)
	widget.SetTitleColor(style.FgColor)
	widget.SetBorder(true)

	widget.SetFocusFunc(func() {
		widget.SetSelectable(true, false)
		widget.ResetSelector()
	})
	widget.SetBlurFunc(func() {
		widget.SetSelectable(false, false)
	})

	return widget
}

func (widget *InterfacesWidget) SetData(data []*pb.Session) {
	widget.Clear()

	widget.data = nil
	for _, session := range data {
		for _, iface := range session.Interfaces {
			widget.data = append(widget.data, iface)
		}
	}

	widget.Refresh()
}

func (widget *InterfacesWidget) ResetSelector() {
	if len(widget.data) > 0 {
		widget.Select(1, 0) // forcing selection for highlighting to work immediately
	}
}

func (widget *InterfacesWidget) Refresh() {
	headers := []string{"Name", "IP"}
	for i := 0; i < len(headers); i++ {
		header := fmt.Sprintf("[::b]%s", strings.ToUpper(headers[i]))
		widget.SetCell(0, i, tview.NewTableCell(header).SetExpansion(1).SetSelectable(false)).SetFixed(1, 0)
	}

	rowId := 1
	for _, elem := range widget.data {
		for _, IP := range elem.IPs {
			widget.SetCell(rowId, 0, tview.NewTableCell(elem.Name))
			widget.SetCell(rowId, 1, tview.NewTableCell(IP))

			rowId++
		}
	}
}
