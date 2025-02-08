package forms

import (
	"strings"

	"github.com/rivo/tview"
)

var (
	redirector_from = FormVal[string]{
		Hint: "Address to listen to on the agent.\n\nExample:\n0.0.0.0:1337",
	}

	redirector_to = FormVal[string]{
		Hint: "Address to send traffic to.\n\nExample:\n1.2.3.4:7331",
	}

	redirector_protocol = FormVal[FormSelectVal]{
		Hint: "Network protocol to use",
	}
)

type AddRedirectorForm struct {
	tview.Flex
	form      *tview.Form
	submitBtn *tview.Button
	cancelBtn *tview.Button
}

func NewAddRedirectorForm() *AddRedirectorForm {
	page := &AddRedirectorForm{
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

	page.form.SetTitle("Add redirector").SetTitleAlign(tview.AlignCenter)
	page.form.SetBorder(true)
	page.form.SetButtonsAlign(tview.AlignCenter)

	fromField := tview.NewInputField()
	fromField.SetLabel("From")
	fromField.SetText(redirector_from.Last)
	fromField.SetFocusFunc(func() {
		hintBox.SetText(redirector_from.Hint)
	})
	fromField.SetChangedFunc(func(text string) {
		redirector_from.Last = text
	})
	page.form.AddFormItem(fromField)

	toField := tview.NewInputField()
	toField.SetLabel("From")
	toField.SetText(redirector_to.Last)
	toField.SetFocusFunc(func() {
		hintBox.SetText(redirector_to.Hint)
	})
	toField.SetChangedFunc(func(text string) {
		redirector_to.Last = text
	})
	page.form.AddFormItem(toField)

	protocolField := tview.NewDropDown()
	protocolField.SetLabel("Protocol")
	protocolField.SetFocusFunc(func() {
		hintBox.SetText(redirector_protocol.Hint)
	})
	protocolField.SetOptions([]string{"TCP", "UDP"}, func(option string, index int) {
		redirector_protocol.Last.ID = index
		redirector_protocol.Last.Value = strings.ToLower(option)
	})
	protocolField.SetBlurFunc(func() {
		hintBox.Clear()
	})
	protocolField.SetCurrentOption(redirector_protocol.Last.ID)
	page.form.AddFormItem(protocolField)

	page.form.AddButton("Submit", nil)
	page.form.AddButton("Cancel", nil)

	formFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(page.form, 11, 1, true).
		AddItem(hintBox, 8, 1, false)

	page.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(formFlex, 0, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)

	return page
}

func (page *AddRedirectorForm) GetID() string {
	return "addroute_page"
}

func (page *AddRedirectorForm) SetSubmitFunc(f func(string, string, string)) {
	btnId := page.form.GetButtonIndex("Submit")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(redirector_from.Last, redirector_to.Last, redirector_protocol.Last.Value)
	})
}

func (page *AddRedirectorForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
