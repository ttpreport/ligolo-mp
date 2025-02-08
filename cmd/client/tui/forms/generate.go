package forms

import (
	"strings"

	"github.com/rivo/tview"
)

var (
	generate_saveTo = FormVal[string]{
		Last: wd,
		Hint: "Path to save the agent binary. Specify only directory for default filename, otherwise provide full path with filename.\n\nExample:\n/home/kali\n/home/kali/myagent.exe",
	}

	generate_servers = FormVal[string]{
		Hint: "Server list, one per line - they are tried sequentially until a successful connection.\n\nExample:\n1.3.3.7:11601\n7.3.3.1:1234\n8.0.0.85:1337",
	}

	generate_proxy = FormVal[string]{
		Hint: "Will override environment variables configuration. Supported protocols: socks5, socks5h, http, https.\n\nExample:\nhttp://proxy:8080\nsocks5://1.2.3.4:3128",
	}

	generate_ignoreEnvProxy = FormVal[bool]{
		Hint: "If checked, will ignore proxies configured via environment variables and attempt a direct connection.",
	}

	generate_goos = FormVal[FormSelectVal]{
		Hint: "Target operating system",
	}

	generate_goarch = FormVal[FormSelectVal]{
		Hint: "Target architecture",
	}

	generate_obfuscate = FormVal[bool]{
		Hint: "Produces obfuscated binary instead of a regular one - might help with AV evasion.",
	}
)

type GenerateForm struct {
	tview.Flex
	form      *tview.Form
	submitBtn *tview.Button
	cancelBtn *tview.Button
}

func NewGenerateForm() *GenerateForm {
	gen := &GenerateForm{
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

	gen.form.SetTitle("Generate agent").SetTitleAlign(tview.AlignCenter)
	gen.form.SetBorder(true)
	gen.form.SetButtonsAlign(tview.AlignCenter)

	saveToField := tview.NewInputField()
	saveToField.SetLabel("Save to")
	saveToField.SetText(generate_saveTo.Last)
	saveToField.SetFocusFunc(func() {
		hintBox.SetText(generate_saveTo.Hint)
	})
	saveToField.SetChangedFunc(func(text string) {
		generate_saveTo.Last = text
	})
	gen.form.AddFormItem(saveToField)

	serversField := tview.NewTextArea()
	serversField.SetLabel("Servers")
	serversField.SetText(generate_servers.Last, true)
	serversField.SetFocusFunc(func() {
		hintBox.SetText(generate_servers.Hint)
	})
	serversField.SetChangedFunc(func() {
		generate_servers.Last = serversField.GetText()
	})
	gen.form.AddFormItem(serversField)

	proxyField := tview.NewInputField()
	proxyField.SetLabel("Proxy")
	proxyField.SetText(generate_proxy.Last)
	proxyField.SetFocusFunc(func() {
		hintBox.SetText(generate_proxy.Hint)
	})
	proxyField.SetChangedFunc(func(text string) {
		generate_proxy.Last = text
	})
	gen.form.AddFormItem(proxyField)

	ignoreEnvProxyField := tview.NewCheckbox()
	ignoreEnvProxyField.SetLabel("Ignore env proxy")
	ignoreEnvProxyField.SetChecked(generate_ignoreEnvProxy.Last)
	ignoreEnvProxyField.SetFocusFunc(func() {
		hintBox.SetText(generate_ignoreEnvProxy.Hint)
	})
	ignoreEnvProxyField.SetChangedFunc(func(checked bool) {
		generate_ignoreEnvProxy.Last = checked
	})
	gen.form.AddFormItem(ignoreEnvProxyField)

	goosField := tview.NewDropDown()
	goosField.SetLabel("OS")
	goosField.SetFocusFunc(func() {
		hintBox.SetText(generate_goos.Hint)
	})
	goosField.SetOptions([]string{"Windows", "Linux", "Darwin"}, func(option string, index int) {
		generate_goos.Last.ID = index
		generate_goos.Last.Value = strings.ToLower(option)
	})
	goosField.SetCurrentOption(generate_goos.Last.ID)
	gen.form.AddFormItem(goosField)

	goarchField := tview.NewDropDown()
	goarchField.SetLabel("Arch")
	goarchField.SetFocusFunc(func() {
		hintBox.SetText(generate_goarch.Hint)
	})
	goarchField.SetOptions([]string{"amd64", "x86", "arm64", "arm"}, func(option string, index int) {
		generate_goarch.Last.ID = index
		generate_goarch.Last.Value = option
	})
	goarchField.SetCurrentOption(generate_goarch.Last.ID)
	gen.form.AddFormItem(goarchField)

	obfuscateField := tview.NewCheckbox()
	obfuscateField.SetLabel("Obfuscate")
	obfuscateField.SetChecked(generate_obfuscate.Last)
	obfuscateField.SetFocusFunc(func() {
		hintBox.SetText(generate_obfuscate.Hint)
	})
	obfuscateField.SetBlurFunc(func() {
		hintBox.Clear()
	})
	obfuscateField.SetChangedFunc(func(checked bool) {
		generate_obfuscate.Last = checked
	})
	gen.form.AddFormItem(obfuscateField)

	gen.form.AddButton("Submit", nil)
	gen.form.AddButton("Cancel", nil)

	formFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(gen.form, 23, 1, true).
		AddItem(hintBox, 11, 1, false)

	gen.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(formFlex, 0, 1, true).
			AddItem(nil, 0, 1, false), 0, 3, true).
		AddItem(nil, 0, 1, false)

	return gen
}

func (form *GenerateForm) GetID() string {
	return "generate_page"
}

func (form *GenerateForm) SetSubmitFunc(f func(path string, servers string, os string, arch string, obfuscate bool, proxy string, ignoreEnvProxy bool)) {
	btnId := form.form.GetButtonIndex("Submit")
	submitBtn := form.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(func() {
		f(
			generate_saveTo.Last,
			generate_servers.Last,
			generate_goos.Last.Value,
			generate_goarch.Last.Value,
			generate_obfuscate.Last,
			generate_proxy.Last,
			generate_ignoreEnvProxy.Last,
		)
	})
}

func (form *GenerateForm) SetCancelFunc(f func()) {
	btnId := form.form.GetButtonIndex("Cancel")
	submitBtn := form.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
