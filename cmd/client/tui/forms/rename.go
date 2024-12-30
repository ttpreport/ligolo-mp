package forms

import (
	"github.com/rivo/tview"
)

type RenameForm struct {
	tview.Flex
	form      *tview.Form
	alias     string
	path      string
	submitBtn *tview.Button
	cancelBtn *tview.Button
}

func NewRenameForm() *RenameForm {
	ren := &RenameForm{
		Flex:      *tview.NewFlex(),
		form:      tview.NewForm(),
		submitBtn: tview.NewButton("Submit"),
		cancelBtn: tview.NewButton("Cancel"),
	}

	ren.form.SetTitle("Rename session").SetTitleAlign(tview.AlignCenter)
	ren.form.SetBorder(true)
	ren.form.SetButtonsAlign(tview.AlignCenter)

	ren.form.AddInputField(
		"Alias",
		"",
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			ren.alias = text
		})

	ren.form.AddButton("Submit", nil)
	ren.form.AddButton("Cancel", nil)

	ren.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(ren.form, 7, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)

	return ren
}

func (page *RenameForm) GetID() string {
	return "rename_page"
}

func (page *RenameForm) SetSubmitFunc(f func(string)) {
	btnId := page.form.GetButtonIndex("Submit")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(page.alias)
	})
}

func (page *RenameForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
