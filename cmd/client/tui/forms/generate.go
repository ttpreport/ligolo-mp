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

	gen.form.AddInputField(
		"Server",
		"",
		0,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			gen.req.Server = text
		})

	gen.form.AddDropDown("OS", []string{"Windows", "Linux", "Darwin"}, 0, func(option string, _ int) {
		gen.req.GOOS = strings.ToLower(option)
	})

	gen.form.AddDropDown("Arch", []string{"amd64", "x86"}, 0, func(option string, _ int) {
		gen.req.GOARCH = option
	})

	gen.form.AddCheckbox("Obfuscate", false, func(checked bool) {
		gen.req.Obfuscate = checked
	})

	gen.form.AddInputField("Save to", "", 0, func(textToCheck string, lastChar rune) bool {
		return true
	}, func(text string) {
		gen.path = text
	})

	gen.form.AddButton("Submit", nil)
	gen.form.AddButton("Cancel", nil)
	//TODO: socks proxy settings

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
