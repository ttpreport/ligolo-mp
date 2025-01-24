package forms

import (
	"github.com/rivo/tview"
)

var operator_name string
var operator_server string
var operator_isAdmin bool

func init() {
	operator_name = ""
	operator_server = ""
	operator_isAdmin = false
}

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

		name:    operator_name,
		server:  operator_server,
		isAdmin: operator_isAdmin,
	}

	gen.form.SetTitle("New operator").SetTitleAlign(tview.AlignCenter)
	gen.form.SetBorder(true)
	gen.form.SetButtonsAlign(tview.AlignCenter)

	gen.form.AddInputField("Name", operator_name, 0, func(textToCheck string, lastChar rune) bool {
		return true
	}, func(text string) {
		gen.name = text
		operator_name = text
	})

	gen.form.AddInputField("Server", operator_server, 0, func(textToCheck string, lastChar rune) bool {
		return true
	}, func(text string) {
		gen.server = text
		operator_server = text
	})

	gen.form.AddCheckbox("Admin", operator_isAdmin, func(checked bool) {
		gen.isAdmin = checked
		operator_isAdmin = checked
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
