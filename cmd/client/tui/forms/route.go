package forms

import (
	"github.com/rivo/tview"
)

type AddRouteForm struct {
	tview.Flex
	form      *tview.Form
	cidr      string
	loopback  bool
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

	form.form.SetTitle("Add route").SetTitleAlign(tview.AlignCenter)
	form.form.SetBorder(true)
	form.form.SetButtonsAlign(tview.AlignCenter)

	form.form.AddInputField(
		"CIDR",
		"",
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			form.cidr = text
		},
	)
	form.form.AddCheckbox(
		"Loopback",
		false,
		func(checked bool) {
			form.loopback = checked
		},
	)

	form.form.AddButton("Submit", nil)
	form.form.AddButton("Cancel", nil)

	form.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form.form, 9, 1, true).
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
		f(page.cidr, page.loopback)
	})
}

func (page *AddRouteForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
