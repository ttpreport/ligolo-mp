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
	ren := &AddRouteForm{
		Flex:      *tview.NewFlex(),
		form:      tview.NewForm(),
		submitBtn: tview.NewButton("Submit"),
		cancelBtn: tview.NewButton("Cancel"),
	}

	ren.form.SetTitle("Rename session").SetTitleAlign(tview.AlignCenter)
	ren.form.SetBorder(true)
	ren.form.SetButtonsAlign(tview.AlignCenter)

	ren.form.AddInputField(
		"CIDR",
		"",
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			ren.cidr = text
		},
	)
	ren.form.AddCheckbox(
		"Loopback",
		false,
		func(checked bool) {
			ren.loopback = checked
		},
	)

	ren.form.AddButton("Submit", nil)
	ren.form.AddButton("Cancel", nil)

	ren.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(ren.form, 9, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)

	return ren
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
