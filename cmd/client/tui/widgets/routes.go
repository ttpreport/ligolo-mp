package widgets

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/utils"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type RoutesWidget struct {
	tview.Table
	data            []*RoutesWidgetElem
	selectedSession *pb.Session
	selectedFunc    func(string)
}

func NewRoutesWidget() *RoutesWidget {
	widget := &RoutesWidget{
		Table: *tview.NewTable(),
		data:  nil,
	}

	widget.Table.SetSelectable(false, false)
	widget.Table.SetBackgroundColor(style.BgColor)
	widget.Table.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("routes")))
	widget.Table.SetBorderColor(style.BorderColor)
	widget.Table.SetTitleColor(style.FgColor)
	widget.Table.SetBorder(true)

	widget.SetFocusFunc(func() {
		widget.SetSelectable(true, false)
		widget.ResetSelector()
	})
	widget.SetBlurFunc(func() {
		widget.SetSelectable(false, false)
	})

	return widget
}

func (widget *RoutesWidget) SetSelectedSession(sess *pb.Session) {
	widget.selectedSession = sess
	widget.Refresh()
}

func (widget *RoutesWidget) FetchElem(row int) *RoutesWidgetElem {
	id := max(0, row-1)
	if len(widget.data) > id {
		return widget.data[id]
	}

	return nil
}

func (widget *RoutesWidget) FetchRow(sess *pb.Session) int {
	for row, elem := range widget.data {
		if elem.Session.ID == sess.ID {
			return row + 1
		}
	}

	return 1
}

func (widget *RoutesWidget) SetSelectionChangedFunc(f func(*RoutesWidgetElem)) {
	widget.Table.SetSelectionChangedFunc(func(row, _ int) {
		item := widget.FetchElem(row)
		if item != nil {
			f(item)
		}
	})
}

func (widget *RoutesWidget) SetSelectedFunc(f func(*RoutesWidgetElem)) {
	widget.Table.SetSelectedFunc(func(row, _ int) {
		item := widget.FetchElem(row)
		if item != nil {
			f(item)
		}
	})
}

func (widget *RoutesWidget) SetData(data []*pb.Session) {
	widget.Clear()

	widget.data = nil

	for _, session := range data {
		for _, route := range session.Tun.Routes {
			widget.data = append(widget.data, NewRoutesWidgetElem(route, session))
		}
	}

	widget.Refresh()
}

func (widget *RoutesWidget) ResetSelector() {
	if len(widget.data) > 0 {
		row := 1
		if widget.selectedSession != nil {
			row = widget.FetchRow(widget.selectedSession)
		}

		widget.Select(row, 0)
	}
}

func (widget *RoutesWidget) Refresh() {
	headers := []string{"Session", "Route", "Loopback", ""}
	for i := 0; i < len(headers); i++ {
		header := fmt.Sprintf("[::b]%s", strings.ToUpper(headers[i]))
		widget.SetCell(0, i, tview.NewTableCell(header).SetExpansion(1).SetSelectable(false)).SetFixed(1, 0)
	}

	rowId := 1
	for _, elem := range widget.data {
		if elem.IsSelected(widget.selectedSession) {
			elem.Highlight(true)
		} else {
			elem.Highlight(false)
		}

		widget.SetCell(rowId, 0, elem.Name())
		widget.SetCell(rowId, 1, elem.Cidr())
		widget.SetCell(rowId, 2, elem.IsLoopback())
		widget.SetCell(rowId, 3, elem.Status().SetSelectable(false).SetAlign(tview.AlignCenter))

		rowId++

	}
}

type RoutesWidgetElem struct {
	Route   *pb.Route
	Session *pb.Session
	bgcolor tcell.Color
}

func NewRoutesWidgetElem(route *pb.Route, session *pb.Session) *RoutesWidgetElem {
	return &RoutesWidgetElem{
		Route:   route,
		Session: session,
		bgcolor: style.BgColor,
	}
}

func (elem *RoutesWidgetElem) IsSelected(sess *pb.Session) bool {
	if sess == nil {
		return false
	}

	if sess.ID != elem.Session.ID {
		return false
	}

	return true
}

func (elem *RoutesWidgetElem) Highlight(h bool) {
	if h {
		elem.bgcolor = style.HighlightColor
	} else {
		elem.bgcolor = style.BgColor
	}
}

func (elem *RoutesWidgetElem) Name() *tview.TableCell {
	val := elem.Session.Hostname
	if elem.Session.Alias != "" {
		val = elem.Session.Alias
	}

	return tview.NewTableCell(val).SetBackgroundColor(elem.bgcolor)
}

func (elem *RoutesWidgetElem) IsLoopback() *tview.TableCell {
	val := utils.HumanBool(elem.Route.IsLoopback)
	return tview.NewTableCell(val).SetBackgroundColor(elem.bgcolor)
}

func (elem *RoutesWidgetElem) Cidr() *tview.TableCell {
	val := elem.Route.Cidr
	return tview.NewTableCell(val).SetBackgroundColor(elem.bgcolor)
}

func (elem *RoutesWidgetElem) Status() *tview.TableCell {
	val := "⚑"
	if !elem.Session.IsConnected {
		return tview.NewTableCell(val).SetTextColor(tcell.ColorRed)
	}

	if !elem.Session.IsRelaying {
		return tview.NewTableCell(val).SetTextColor(tcell.ColorBlue)
	}

	return tview.NewTableCell(val).SetTextColor(tcell.ColorGreen)
}
