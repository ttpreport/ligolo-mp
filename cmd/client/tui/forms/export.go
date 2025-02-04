package forms

import (
	"os"

	"github.com/rivo/tview"
)

var export_lastFolder string

func init() {
	export_lastFolder, _ = os.Getwd()
}

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

		path: export_lastFolder,
	}

	export.form.SetTitle("Export operator").SetTitleAlign(tview.AlignCenter)
	export.form.SetBorder(true)
	export.form.SetButtonsAlign(tview.AlignCenter)

	export.form.AddInputField(
		"Save to",
		export_lastFolder,
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			export.path = text
			export_lastFolder = text
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
