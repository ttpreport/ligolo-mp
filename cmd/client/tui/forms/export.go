package forms

import (
	"github.com/rivo/tview"
)

var (
	export_saveTo = FormVal[string]{
		Last: wd,
		Hint: "Path to save the operator connection file. Specify only directory for default filename, otherwise provide full path with filename.\n\nExample:\n/home/kali\n/home/kali/operator.json",
	}
)

type ExportForm struct {
	tview.Flex
	form      *tview.Form
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

	hintBox := tview.NewTextView()
	hintBox.SetTitle("HINT")
	hintBox.SetTitleAlign(tview.AlignCenter)
	hintBox.SetBorder(true)
	hintBox.SetBorderPadding(1, 1, 1, 1)

	export.form.SetTitle("Export operator").SetTitleAlign(tview.AlignCenter)
	export.form.SetBorder(true)
	export.form.SetButtonsAlign(tview.AlignCenter)

	saveToField := tview.NewInputField()
	saveToField.SetLabel("Save to")
	saveToField.SetText(export_saveTo.Last)
	saveToField.SetFocusFunc(func() {
		hintBox.SetText(export_saveTo.Hint)
	})
	saveToField.SetChangedFunc(func(text string) {
		export_saveTo.Last = text
	})
	saveToField.SetBlurFunc(func() {
		hintBox.Clear()
	})
	export.form.AddFormItem(saveToField)

	export.form.AddButton("Submit", nil)
	export.form.AddButton("Cancel", nil)

	formFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(export.form, 7, 1, true).
		AddItem(hintBox, 11, 1, false)

	export.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(formFlex, 0, 1, true).
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
		f(export_saveTo.Last)
	})
}

func (page *ExportForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
