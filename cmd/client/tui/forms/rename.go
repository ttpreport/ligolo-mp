package forms

import (
	"github.com/rivo/tview"
)

var (
	rename_alias = FormVal[string]{
		Hint: "New session alias",
	}
)

type RenameForm struct {
	tview.Flex
	form      *tview.Form
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

	hintBox := tview.NewTextView()
	hintBox.SetTitle("HINT")
	hintBox.SetTitleAlign(tview.AlignCenter)
	hintBox.SetBorder(true)
	hintBox.SetBorderPadding(1, 1, 1, 1)

	ren.form.SetTitle("Rename session").SetTitleAlign(tview.AlignCenter)
	ren.form.SetBorder(true)
	ren.form.SetButtonsAlign(tview.AlignCenter)

	aliasField := tview.NewInputField()
	aliasField.SetLabel("Alias")
	aliasField.SetText(rename_alias.Last)
	aliasField.SetFocusFunc(func() {
		hintBox.SetText(rename_alias.Hint)
	})
	aliasField.SetChangedFunc(func(text string) {
		rename_alias.Last = text
	})
	aliasField.SetBlurFunc(func() {
		hintBox.Clear()
	})
	ren.form.AddFormItem(aliasField)

	ren.form.AddButton("Submit", nil)
	ren.form.AddButton("Cancel", nil)

	formFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ren.form, 7, 1, true).
		AddItem(hintBox, 5, 1, false)

	ren.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(formFlex, 0, 1, true).
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
		f(rename_alias.Last)
	})
}

func (page *RenameForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
