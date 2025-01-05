package forms

import (
	"github.com/rivo/tview"
)

type OperatorForm struct {
	tview.Flex
	form *tview.Form

	name    string
	isAdmin bool
	server  string

	submitBtn *tview.Button
	cancelBtn *tview.Button
}

func NewOperatorForm() *OperatorForm {
	gen := &OperatorForm{
		Flex:      *tview.NewFlex(),
		form:      tview.NewForm(),
		submitBtn: tview.NewButton("Submit"),
		cancelBtn: tview.NewButton("Cancel"),
	}

	gen.form.SetTitle("New operator").SetTitleAlign(tview.AlignCenter)
	gen.form.SetBorder(true)
	gen.form.SetButtonsAlign(tview.AlignCenter)

	gen.form.AddInputField("Name", "", 0, func(textToCheck string, lastChar rune) bool {
		return true
	}, func(text string) {
		gen.name = text
	})

	gen.form.AddInputField("Server", "", 0, func(textToCheck string, lastChar rune) bool {
		return true
	}, func(text string) {
		gen.server = text
	})

	gen.form.AddCheckbox("Admin", false, func(checked bool) {
		gen.isAdmin = checked
	})

	gen.form.AddButton("Submit", nil)
	gen.form.AddButton("Cancel", nil)

	gen.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(gen.form, 11, 1, true).
			AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 0, 1, false)

	return gen
}

func (page *OperatorForm) GetID() string {
	return "operator_form"
}

func (page *OperatorForm) SetSubmitFunc(f func(string, bool, string)) {
	btnId := page.form.GetButtonIndex("Submit")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(page.name, page.isAdmin, page.server)
	})
}

func (page *OperatorForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
