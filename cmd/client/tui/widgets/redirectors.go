package widgets

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type RedirectorsWidget struct {
	tview.Table
	data            []*RedirectorsWidgetElem
	selectedSession *pb.Session
	selectedFunc    func(string)
}

func NewRedirectorsWidget() *RedirectorsWidget {
	widget := &RedirectorsWidget{
		Table: *tview.NewTable(),
		data:  nil,
	}

	widget.Table.SetSelectable(false, false)
	widget.Table.SetBackgroundColor(style.BgColor)
	widget.Table.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("redirectors")))
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

func (widget *RedirectorsWidget) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()
		switch key {
		default:
			defaultHandler := widget.Table.InputHandler()
			defaultHandler(event, setFocus)
		}
	}

}

func (widget *RedirectorsWidget) SetData(data []*pb.Session) {
	widget.Clear()

	widget.data = nil
	for _, session := range data {
		for _, redirector := range session.Redirectors {
			widget.data = append(widget.data, NewRedirectorsWidgetElem(redirector, session))
		}
	}

	widget.Refresh()
	widget.ResetSelector()
}

func (widget *RedirectorsWidget) SetSelectedSession(sess *pb.Session) {
	widget.selectedSession = sess
	widget.Refresh()
}

func (widget *RedirectorsWidget) FetchElem(row int) *RedirectorsWidgetElem {
	id := max(0, row-1)
	if len(widget.data) > id {
		return widget.data[id]
	}

	return nil
}

func (widget *RedirectorsWidget) FetchRow(sess *pb.Session) int {
	for row, elem := range widget.data {
		if elem.Session.ID == sess.ID {
			return row + 1
		}
	}

	return 0
}

func (widget *RedirectorsWidget) SetSelectionChangedFunc(f func(*RedirectorsWidgetElem)) {
	widget.Table.SetSelectionChangedFunc(func(row, _ int) {
		item := widget.FetchElem(row)
		if item != nil {
			f(item)
		}
	})
}

func (widget *RedirectorsWidget) SetSelectedFunc(f func(*RedirectorsWidgetElem)) {
	widget.Table.SetSelectedFunc(func(row, _ int) {
		item := widget.FetchElem(row)
		if item != nil {
			f(item)
		}
	})
}

func (widget *RedirectorsWidget) ResetSelector() {
	if len(widget.data) > 0 {
		row := 1
		if widget.selectedSession != nil {
			row = widget.FetchRow(widget.selectedSession)
		}

		if row > 0 {
			widget.Select(row, 0)
		}
	}
}

func (widget *RedirectorsWidget) Refresh() {
	headers := []string{"Session", "From", "To", "Protocol", ""}
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
		widget.SetCell(rowId, 1, elem.From())
		widget.SetCell(rowId, 2, elem.To())
		widget.SetCell(rowId, 3, elem.Protocol())
		widget.SetCell(rowId, 4, elem.Status().SetSelectable(false).SetAlign(tview.AlignCenter))

		rowId++

	}
}

type RedirectorsWidgetElem struct {
	Redirector *pb.Redirector
	Session    *pb.Session
	bgcolor    tcell.Color
}

func NewRedirectorsWidgetElem(redirector *pb.Redirector, session *pb.Session) *RedirectorsWidgetElem {
	return &RedirectorsWidgetElem{
		Redirector: redirector,
		Session:    session,
		bgcolor:    style.BgColor,
	}
}

func (elem *RedirectorsWidgetElem) Name() *tview.TableCell {
	val := elem.Session.Hostname
	if elem.Session.Alias != "" {
		val = elem.Session.Alias
	}

	return tview.NewTableCell(val).SetBackgroundColor(elem.bgcolor)
}

func (elem *RedirectorsWidgetElem) IsSelected(sess *pb.Session) bool {
	if sess == nil {
		return false
	}

	if sess.ID != elem.Session.ID {
		return false
	}

	return true
}

func (elem *RedirectorsWidgetElem) Highlight(h bool) {
	if h {
		elem.bgcolor = style.HighlightColor
	} else {
		elem.bgcolor = style.BgColor
	}
}

func (elem *RedirectorsWidgetElem) From() *tview.TableCell {
	val := elem.Redirector.From
	return tview.NewTableCell(val).SetBackgroundColor(elem.bgcolor)
}

func (elem *RedirectorsWidgetElem) To() *tview.TableCell {
	val := elem.Redirector.To
	return tview.NewTableCell(val).SetBackgroundColor(elem.bgcolor)
}

func (elem *RedirectorsWidgetElem) Protocol() *tview.TableCell {
	val := elem.Redirector.Protocol
	return tview.NewTableCell(strings.ToUpper(val)).SetBackgroundColor(elem.bgcolor)
}

func (elem *RedirectorsWidgetElem) Status() *tview.TableCell {
	val := "⚑"
	if !elem.Session.IsConnected {
		return tview.NewTableCell(val).SetTextColor(tcell.ColorRed)
	}

	if !elem.Session.IsRelaying {
		return tview.NewTableCell(val).SetTextColor(tcell.ColorBlue)
	}

	return tview.NewTableCell(val).SetTextColor(tcell.ColorGreen)
}
