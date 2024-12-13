package forms

import (
	"strings"

	"github.com/rivo/tview"
)

type AddRedirectorForm struct {
	tview.Flex
	form      *tview.Form
	from      string
	to        string
	proto     string
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

	page.form.SetTitle("Rename session").SetTitleAlign(tview.AlignCenter)
	page.form.SetBorder(true)
	page.form.SetButtonsAlign(tview.AlignCenter)

	page.form.AddInputField(
		"From",
		"",
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			page.from = text
		},
	)
	page.form.AddInputField(
		"To",
		"",
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			page.to = text
		},
	)
	page.form.AddDropDown("Protocol", []string{"TCP", "UDP"}, 0, func(option string, _ int) {
		page.proto = strings.ToLower(option)
	})

	page.form.AddButton("Submit", nil)
	page.form.AddButton("Cancel", nil)

	page.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(page.form, 11, 1, true).
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
		f(page.from, page.to, page.proto)
	})
}

func (page *AddRedirectorForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
