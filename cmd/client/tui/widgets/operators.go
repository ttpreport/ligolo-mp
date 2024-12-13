package widgets

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/utils"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type OperatorsWidget struct {
	*tview.Table
	data []*OperatorsWidgetElem
}

func NewOperatorsWidget() *OperatorsWidget {
	widget := &OperatorsWidget{
		Table: tview.NewTable(),
	}

	widget.SetSelectable(false, false)
	widget.SetBackgroundColor(style.BgColor)
	widget.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("operators")))
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

func (widget *OperatorsWidget) SetData(data []*pb.Operator) {
	widget.Clear()

	widget.data = nil
	for _, elem := range data {
		widget.data = append(widget.data, NewOperatorsWidgetElem(elem))
	}

	widget.Refresh()
	widget.ResetSelector()
}

func (widget *OperatorsWidget) FetchElem(row int) *OperatorsWidgetElem {
	id := max(0, row-1)
	if len(widget.data) > id {
		return widget.data[id]
	}

	return nil
}

func (widget *OperatorsWidget) ResetSelector() {
	if len(widget.data) > 0 {
		widget.Select(1, 0) // forcing selection for highlighting to work immediately
	}
}

func (widget *OperatorsWidget) Refresh() {
	headers := []string{"Name", "Is Admin", "Online"}
	for i := 0; i < len(headers); i++ {
		header := fmt.Sprintf("[::b]%s", strings.ToUpper(headers[i]))
		widget.SetCell(0, i, tview.NewTableCell(header).SetExpansion(1).SetSelectable(false)).SetFixed(1, 0)
	}

	rowId := 1
	for _, elem := range widget.data {
		widget.SetCell(rowId, 0, elem.Name())
		widget.SetCell(rowId, 1, elem.IsAdmin())
		widget.SetCell(rowId, 2, elem.IsOnline())

		rowId++
	}
}

func (widget *OperatorsWidget) SetSelectedFunc(f func(*OperatorsWidgetElem)) {
	widget.Table.SetSelectedFunc(func(row, _ int) {
		item := widget.FetchElem(row)
		if item != nil {
			f(item)
		}
	})
}

type OperatorsWidgetElem struct {
	Operator *pb.Operator
}

func NewOperatorsWidgetElem(operator *pb.Operator) *OperatorsWidgetElem {
	return &OperatorsWidgetElem{
		Operator: operator,
	}
}

func (elem *OperatorsWidgetElem) Name() *tview.TableCell {
	return tview.NewTableCell(elem.Operator.Name)
}

func (elem *OperatorsWidgetElem) IsAdmin() *tview.TableCell {
	val := utils.HumanBool(elem.Operator.IsAdmin)
	return tview.NewTableCell(val)
}

func (elem *OperatorsWidgetElem) IsOnline() *tview.TableCell {
	val := utils.HumanBool(elem.Operator.IsOnline)
	return tview.NewTableCell(val)
}
