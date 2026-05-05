package modals

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/style"
)

type MenuModal struct {
	tview.Flex
	title      string
	table      *tview.Table
	data       []MenuModalElem
	cancelFunc func()
}

type MenuModalElem struct {
	title      string
	actionFunc func()
}

func NewMenuModal(title string) *MenuModal {
	menu := &MenuModal{
		Flex:  *tview.NewFlex(),
		title: title,
		table: tview.NewTable(),
		data:  nil,
	}

	menu.table.SetSelectable(true, false)
	menu.table.SetBackgroundColor(style.ModalBgColor)
	menu.table.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper(menu.title)))
	menu.table.SetBorderColor(style.BorderColor)
	menu.table.SetTitleColor(style.FgColor)
	menu.table.SetBorder(true)

	menu.table.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			x, y := event.Position()
			tableX, tableY, tableWidth, tableHeight := menu.table.GetInnerRect()
			if x >= tableX && x < tableX+tableWidth && y >= tableY && y < tableY+tableHeight {
				clickedRow := y - tableY
				if clickedRow >= 0 && clickedRow < len(menu.data) {
					menu.data[clickedRow].actionFunc()
					return action, nil
				}
			}
		}
		return action, event
	})

	menu.Flex.AddItem(nil, 0, 2, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 2, false).
			AddItem(menu.table, 0, 1, true).
			AddItem(nil, 0, 2, false),
			0, 1, true).
		AddItem(nil, 0, 2, false)

	menu.table.SetSelectedFunc(func(id, _ int) {
		item := menu.data[id]
		item.actionFunc()
	})

	return menu
}

func NewMenuModalElem(title string, actionFunc func()) MenuModalElem {
	return MenuModalElem{
		title:      title,
		actionFunc: actionFunc,
	}
}

func (page *MenuModal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()
		switch key {
		case tcell.KeyEscape:
			if page.cancelFunc != nil {
				page.cancelFunc()
			}
		default:
			defaultHandler := page.Flex.InputHandler()
			defaultHandler(event, setFocus)
		}
	}

}

func (page *MenuModal) GetID() string {
	return fmt.Sprintf("menu_%s", page.title)
}

func (page *MenuModal) SetCancelFunc(f func()) {
	page.cancelFunc = f
}

func (page *MenuModal) AddItem(elem MenuModalElem) {
	page.data = append(page.data, elem)
	page.table.Clear()

	rowNo := 0
	for _, item := range page.data {
		cell := tview.NewTableCell(item.title).
			SetExpansion(1).
			SetAlign(tview.AlignCenter).
			SetSelectable(true) // Ensure cell is selectable for mouse interaction
		page.table.SetCell(rowNo, 0, cell)
		rowNo++
	}
}

func (page *MenuModal) GetTable() *tview.Table {
	return page.table
}
