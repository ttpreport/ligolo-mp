package forms

import (
	"github.com/rivo/tview"
)

var (
	route_cidr = FormVal[string]{
		Hint: "A CIDR that will be routed via this session.\n\nExample:\n10.10.5.0/24",
	}

	route_loopback = FormVal[bool]{
		Hint: "If checked, specified CIDR will address the machine running the agent itself, i.e. localhost. Use this instead of port forwarding.",
	}
)

type AddRouteForm struct {
	tview.Flex
	form      *tview.Form
	submitBtn *tview.Button
	cancelBtn *tview.Button
}

func NewAddRouteForm() *AddRouteForm {
	form := &AddRouteForm{
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

	form.form.SetTitle("Add route").SetTitleAlign(tview.AlignCenter)
	form.form.SetBorder(true)
	form.form.SetButtonsAlign(tview.AlignCenter)

	cidrField := tview.NewInputField()
	cidrField.SetLabel("CIDR")
	cidrField.SetText(route_cidr.Last)
	cidrField.SetFocusFunc(func() {
		hintBox.SetText(route_cidr.Hint)
	})
	cidrField.SetChangedFunc(func(text string) {
		route_cidr.Last = text
	})
	form.form.AddFormItem(cidrField)

	loopbackField := tview.NewCheckbox()
	loopbackField.SetLabel("Loopback")
	loopbackField.SetChecked(route_loopback.Last)
	loopbackField.SetFocusFunc(func() {
		hintBox.SetText(route_loopback.Hint)
	})
	loopbackField.SetChangedFunc(func(checked bool) {
		route_loopback.Last = checked
	})
	loopbackField.SetBlurFunc(func() {
		hintBox.Clear()
	})
	form.form.AddFormItem(loopbackField)

	form.form.AddButton("Submit", nil)
	form.form.AddButton("Cancel", nil)

	formFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form.form, 9, 1, true).
		AddItem(hintBox, 8, 1, false)

	form.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(formFlex, 0, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)

	return form
}

func (page *AddRouteForm) GetID() string {
	return "addroute_page"
}

func (page *AddRouteForm) SetSubmitFunc(f func(string, bool)) {
	btnId := page.form.GetButtonIndex("Submit")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(route_cidr.Last, route_loopback.Last)
	})
}

func (page *AddRouteForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
