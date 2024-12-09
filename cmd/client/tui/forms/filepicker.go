package forms

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
)

type FilepickerPage struct {
	tview.Flex
	form *tview.Form
	path string
}

func NewFilepickerForm() *FilepickerPage {
	page := &FilepickerPage{
		Flex: *tview.NewFlex(),
		form: tview.NewForm(),
	}

	filepicker := NewFilepicker("picker")
	filepicker.SetSelectedFunc(func(p string) {
		page.path = p
	})

	page.form.SetTitle("Select file").SetTitleAlign(tview.AlignCenter)
	page.form.SetBorder(true)
	page.form.SetButtonsAlign(tview.AlignCenter)
	page.form.AddButton("Select", nil)
	page.form.AddButton("Cancel", nil)
	page.form.AddFormItem(filepicker)

	page.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(page.form, 21, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)

	return page
}

func (page *FilepickerPage) Reset() {
	pickerId := page.form.GetFormItemIndex("picker")
	page.form.SetFocus(pickerId)
}

func (page *FilepickerPage) SetSelectFunc(f func(string)) {
	btnId := page.form.GetButtonIndex("Select")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(page.path)
	})
}

func (page *FilepickerPage) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}

func (picker *FilepickerPage) GetID() string {
	return "filepicker"
}

type Filepicker struct {
	tview.Table
	label        string
	data         []string
	currentPath  string
	finishedFunc func(key tcell.Key)
	selectedFunc func(string)
	cancelFunc   func()
}

func NewFilepicker(label string) *Filepicker {
	picker := &Filepicker{
		Table: *tview.NewTable(),
		label: label,
		data:  nil,
	}

	path, err := os.Getwd()
	if err != nil {
		path = "/"
	}

	picker.currentPath = path
	picker.RefreshData()
	picker.SetSelectable(true, false)
	picker.SetBackgroundColor(style.ModalBgColor)
	picker.SetBorderColor(style.BorderColor)
	picker.SetTitleColor(style.FgColor)
	picker.SetBorder(true)

	picker.Table.SetSelectedFunc(func(id, _ int) {
		item := picker.data[id]
		selectedPath := filepath.Join(picker.currentPath, item)
		finfo, _ := os.Stat(selectedPath)
		if finfo.IsDir() {
			picker.currentPath = selectedPath
			picker.RefreshData()
		} else {
			picker.selectedFunc(selectedPath)
			picker.finishedFunc(tcell.KeyEnter)
		}
	})

	return picker
}

func (picker *Filepicker) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()
		switch key {
		case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyEscape:
			if picker.finishedFunc != nil {
				picker.finishedFunc(key)
			}
		default:
			defaultHandler := picker.Table.InputHandler()
			defaultHandler(event, setFocus)
		}
	}

}

func (picker *Filepicker) SetSelectedFunc(f func(string)) {
	picker.selectedFunc = f
}

func (picker *Filepicker) SetCancelFunc(f func()) {
	picker.cancelFunc = f
}

func (picker *Filepicker) RefreshData() {
	var itemName string
	picker.Clear()
	picker.data = nil

	rowNo := 0
	picker.data = append(picker.data, "..")
	picker.SetCell(rowNo, 0, tview.NewTableCell(">> .."))

	items, _ := os.ReadDir(picker.currentPath)
	for _, item := range items {
		if item.IsDir() {
			rowNo++
			picker.data = append(picker.data, item.Name())
			itemName = fmt.Sprintf(">> %s", item.Name())
			picker.SetCell(rowNo, 0, tview.NewTableCell(itemName))
		}
	}

	for _, item := range items {
		var itemName string
		if !item.IsDir() {
			rowNo++
			picker.data = append(picker.data, item.Name())
			itemName = fmt.Sprintf("%s", item.Name())
			picker.SetCell(rowNo, 0, tview.NewTableCell(itemName))
		}
	}

	picker.ScrollToBeginning().Select(0, 0)
}

func (picker *Filepicker) GetLabel() string {
	return picker.label
}

func (picker *Filepicker) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	return picker
}

func (picker *Filepicker) GetFieldWidth() int {
	return 0
}

func (picker *Filepicker) GetFieldHeight() int {
	return 15
}

func (picker *Filepicker) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	picker.finishedFunc = handler
	return picker
}

func (picker *Filepicker) SetDisabled(disabled bool) tview.FormItem {
	return picker
}
