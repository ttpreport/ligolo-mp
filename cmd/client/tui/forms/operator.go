package forms

import (
	"github.com/rivo/tview"
)

var (
	operator_name = FormVal[string]{
		Hint: "Operator name",
	}

	operator_server = FormVal[string]{
		Hint: "Address of this server with a port.\n\nExample:\n1.2.3.4:58008",
	}

	operator_isAdmin = FormVal[bool]{
		Hint: "If checked, operator will have access to administrative functions",
	}
)

type OperatorForm struct {
	tview.Flex
	form *tview.Form

	submitBtn *tview.Button
	cancelBtn *tview.Button
}

func NewOperatorForm() *OperatorForm {
	form := &OperatorForm{
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

	form.form.SetTitle("New operator").SetTitleAlign(tview.AlignCenter)
	form.form.SetBorder(true)
	form.form.SetButtonsAlign(tview.AlignCenter)

	nameField := tview.NewInputField()
	nameField.SetLabel("Name")
	nameField.SetText(operator_name.Last)
	nameField.SetFocusFunc(func() {
		hintBox.SetText(operator_name.Hint)
	})
	nameField.SetChangedFunc(func(text string) {
		operator_name.Last = text
	})
	form.form.AddFormItem(nameField)

	serverField := tview.NewInputField()
	serverField.SetLabel("Server")
	serverField.SetText(operator_server.Last)
	serverField.SetFocusFunc(func() {
		hintBox.SetText(operator_server.Hint)
	})
	serverField.SetChangedFunc(func(text string) {
		operator_server.Last = text
	})
	form.form.AddFormItem(serverField)

	isAdminField := tview.NewCheckbox()
	isAdminField.SetLabel("Admin")
	isAdminField.SetChecked(operator_isAdmin.Last)
	isAdminField.SetFocusFunc(func() {
		hintBox.SetText(operator_isAdmin.Hint)
	})
	isAdminField.SetChangedFunc(func(checked bool) {
		operator_isAdmin.Last = checked
	})
	isAdminField.SetBlurFunc(func() {
		hintBox.Clear()
	})
	form.form.AddFormItem(isAdminField)

	form.form.AddButton("Submit", nil)
	form.form.AddButton("Cancel", nil)

	formFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form.form, 11, 1, true).
		AddItem(hintBox, 8, 1, false)

	form.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(formFlex, 0, 1, true).
			AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 0, 1, false)

	return form
}

func (page *OperatorForm) GetID() string {
	return "operator_form"
}

func (page *OperatorForm) SetSubmitFunc(f func(string, bool, string)) {
	btnId := page.form.GetButtonIndex("Submit")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(operator_name.Last, operator_isAdmin.Last, operator_server.Last)
	})
}

func (page *OperatorForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
