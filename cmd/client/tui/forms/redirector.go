package forms

import (
	"strings"

	"github.com/rivo/tview"
)

var redirector_from string
var redirector_to string
var redirector_protocolIdx int
var redirector_protocolVal string

func init() {
	redirector_from = ""
	redirector_to = ""
	redirector_protocolIdx = 0
	redirector_protocolVal = ""
}

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

		from:  redirector_from,
		to:    redirector_to,
		proto: redirector_protocolVal,
	}

	page.form.SetTitle("Add redirector").SetTitleAlign(tview.AlignCenter)
	page.form.SetBorder(true)
	page.form.SetButtonsAlign(tview.AlignCenter)

	page.form.AddInputField(
		"From",
		redirector_from,
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			page.from = text
			redirector_from = text
		},
	)
	page.form.AddInputField(
		"To",
		redirector_to,
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			page.to = text
			redirector_to = text
		},
	)
	page.form.AddDropDown("Protocol", []string{"TCP", "UDP"}, redirector_protocolIdx, func(option string, index int) {
		page.proto = strings.ToLower(option)
		redirector_protocolIdx = index
		redirector_protocolVal = strings.ToLower(option)
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
