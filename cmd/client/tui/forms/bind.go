package forms

import (
	"github.com/rivo/tview"
)

var (
	bind_address = FormVal[string]{
		Hint: "Bind agent address (e.g. 192.168.1.10:4444)",
	}
)

type BindAgentForm struct {
	tview.Flex
	form *tview.Form
}

func NewBindAgentForm() *BindAgentForm {
	f := &BindAgentForm{
		Flex: *tview.NewFlex(),
		form: tview.NewForm(),
	}

	hintBox := tview.NewTextView()
	hintBox.SetTitle("HINT")
	hintBox.SetTitleAlign(tview.AlignCenter)
	hintBox.SetBorder(true)
	hintBox.SetBorderPadding(1, 1, 1, 1)

	f.form.SetTitle("Connect to Bind Agent").SetTitleAlign(tview.AlignCenter)
	f.form.SetBorder(true)
	f.form.SetButtonsAlign(tview.AlignCenter)

	addrField := tview.NewInputField()
	addrField.SetLabel("Address")
	addrField.SetText(bind_address.Last)
	addrField.SetFocusFunc(func() { hintBox.SetText(bind_address.Hint) })
	addrField.SetChangedFunc(func(text string) { bind_address.Last = text })
	addrField.SetBlurFunc(func() { hintBox.Clear() })
	f.form.AddFormItem(addrField)

	f.form.AddButton("Connect", nil)
	f.form.AddButton("Cancel", nil)

	formFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(f.form, 7, 1, true).
		AddItem(hintBox, 5, 1, false)

	f.Flex.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(formFlex, 0, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)

	return f
}

func (f *BindAgentForm) GetID() string {
	return "bind_agent_form"
}

func (f *BindAgentForm) SetSubmitFunc(fn func(addr string)) {
	btnId := f.form.GetButtonIndex("Connect")
	f.form.GetButton(btnId).SetSelectedFunc(func() {
		fn(bind_address.Last)
	})
}

func (f *BindAgentForm) SetCancelFunc(fn func()) {
	btnId := f.form.GetButtonIndex("Cancel")
	f.form.GetButton(btnId).SetSelectedFunc(fn)
}
