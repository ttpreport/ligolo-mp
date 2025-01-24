package forms

import (
	"os"
	"strings"

	"github.com/rivo/tview"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

var generate_lastSaveTo string
var generate_lastServers string
var generate_lastProxyServer string
var generate_lastIgnoreEnvProxy bool
var generate_lastOSidx int
var generate_lastOSval string
var generate_lastArchIdx int
var generate_lastArchVal string
var generate_lastObfuscate bool

func init() {
	generate_lastSaveTo, _ = os.Getwd()
	generate_lastServers = ""
	generate_lastProxyServer = ""
	generate_lastIgnoreEnvProxy = false
	generate_lastOSidx = 0
	generate_lastOSval = ""
	generate_lastArchIdx = 0
	generate_lastArchVal = ""
	generate_lastObfuscate = false
}

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

		req: &pb.GenerateAgentReq{
			Servers:        generate_lastServers,
			ProxyServer:    generate_lastProxyServer,
			IgnoreEnvProxy: generate_lastIgnoreEnvProxy,
			GOOS:           generate_lastOSval,
			GOARCH:         generate_lastArchVal,
			Obfuscate:      generate_lastObfuscate,
		},
		path: generate_lastSaveTo,
	}

	gen.form.SetTitle("Generate agent").SetTitleAlign(tview.AlignCenter)
	gen.form.SetBorder(true)
	gen.form.SetButtonsAlign(tview.AlignCenter)

	gen.form.AddInputField("Save to", generate_lastSaveTo, 0, nil,
		func(text string) {
			gen.path = text
			generate_lastSaveTo = text
		},
	)

	gen.form.AddTextArea("Server (one per line)", generate_lastServers, 0, 0, 0,
		func(text string) {
			gen.req.Servers = text
			generate_lastServers = text
		},
	)

	gen.form.AddInputField("Proxy", generate_lastProxyServer, 0, nil,
		func(text string) {
			gen.req.ProxyServer = text
			generate_lastProxyServer = text
		},
	)

	gen.form.AddCheckbox("Ignore env proxy", generate_lastIgnoreEnvProxy, func(checked bool) {
		gen.req.IgnoreEnvProxy = checked
		generate_lastIgnoreEnvProxy = checked
	})

	gen.form.AddDropDown("OS", []string{"Windows", "Linux", "Darwin"}, generate_lastOSidx, func(option string, index int) {
		gen.req.GOOS = strings.ToLower(option)
		generate_lastOSidx = index
		generate_lastOSval = strings.ToLower(option)
	})

	gen.form.AddDropDown("Arch", []string{"amd64", "x86", "arm64", "arm"}, generate_lastArchIdx, func(option string, index int) {
		gen.req.GOARCH = option
		generate_lastArchIdx = index
		generate_lastArchVal = option
	})

	gen.form.AddCheckbox("Obfuscate", generate_lastObfuscate, func(checked bool) {
		gen.req.Obfuscate = checked
		generate_lastObfuscate = checked
	})

	gen.form.AddButton("Submit", nil)
	gen.form.AddButton("Cancel", nil)

	gen.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(gen.form, 23, 1, true).
			AddItem(nil, 0, 1, false), 0, 3, true).
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
