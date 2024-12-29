package forms

import (
	"strings"

	"github.com/rivo/tview"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type GenerateForm struct {
	tview.Flex
	form      *tview.Form
	req       *pb.GenerateAgentReq
	path      string
	submitBtn *tview.Button
	cancelBtn *tview.Button
}

func NewGenerateForm() *GenerateForm {
	gen := &GenerateForm{
		Flex:      *tview.NewFlex(),
		form:      tview.NewForm(),
		submitBtn: tview.NewButton("Submit"),
		cancelBtn: tview.NewButton("Cancel"),
		req:       &pb.GenerateAgentReq{},
	}

	gen.form.SetTitle("Generate agent").SetTitleAlign(tview.AlignCenter)
	gen.form.SetBorder(true)
	gen.form.SetButtonsAlign(tview.AlignCenter)

	gen.form.AddInputField("Save to", "", 0, nil,
		func(text string) {
			gen.path = text
		},
	)

	gen.form.AddTextArea("Server (one per line)", "", 0, 0, 0,
		func(text string) {
			gen.req.Servers = text
		},
	)

	gen.form.AddDropDown("OS", []string{"Windows", "Linux", "Darwin"}, 0, func(option string, _ int) {
		gen.req.GOOS = strings.ToLower(option)
	})

	gen.form.AddDropDown("Arch", []string{"amd64", "x86"}, 0, func(option string, _ int) {
		gen.req.GOARCH = option
	})

	gen.form.AddCheckbox("Obfuscate", false, func(checked bool) {
		gen.req.Obfuscate = checked
	})

	gen.form.AddCheckbox("Use proxy", false, func(checked bool) {
		if checked {
			gen.form.AddInputField("Proxy address", "", 0, nil,
				func(text string) {
					gen.req.ProxyServer = text
				},
			)
		} else {
			gen.form.RemoveFormItem(gen.form.GetFormItemIndex("Proxy host"))
			gen.form.RemoveFormItem(gen.form.GetFormItemIndex("Proxy user"))
			gen.form.RemoveFormItem(gen.form.GetFormItemIndex("Proxy pass"))
		}
	})

	gen.form.AddButton("Submit", nil)
	gen.form.AddButton("Cancel", nil)

	gen.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(gen.form, 15, 1, true).
			AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 0, 1, false)

	return gen
}

func (page *GenerateForm) GetID() string {
	return "generate_page"
}

func (page *GenerateForm) SetSubmitFunc(f func(*pb.GenerateAgentReq, string)) {
	btnId := page.form.GetButtonIndex("Submit")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(page.req, page.path)
	})
}

func (page *GenerateForm) SetCancelFunc(f func()) {
	btnId := page.form.GetButtonIndex("Cancel")
	submitBtn := page.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
