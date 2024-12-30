package forms

import (
	"github.com/rivo/tview"
)

type ExportForm struct {
	tview.Flex
	form      *tview.Form
	path      string
	submitBtn *tview.Button
	cancelBtn *tview.Button
}

func NewExportForm() *ExportForm {
	export := &ExportForm{
		Flex:      *tview.NewFlex(),
		form:      tview.NewForm(),
		submitBtn: tview.NewButton("Submit"),
		cancelBtn: tview.NewButton("Cancel"),
	}

	export.form.SetTitle("Export operator").SetTitleAlign(tview.AlignCenter)
	export.form.SetBorder(true)
	export.form.SetButtonsAlign(tview.AlignCenter)

	export.form.AddInputField(
		"Folder",
		"",
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			export.path = text
		})

	export.form.AddButton("Submit", nil)
	export.form.AddButton("Cancel", nil)

	export.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(export.form, 7, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)

	return export
}

func (page *ExportForm) GetID() string {
	return "export_form"
}

func (page *ExportForm) SetSubmitFunc(f func(string)) {
	btnId := page.form.GetButtonIndex("Submit")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(page.path)
	})
}

func (page *ExportForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
