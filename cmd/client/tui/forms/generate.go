package forms

import (
	"strings"

	"github.com/rivo/tview"
)

var (
	saveTo = FormVal[string]{
		Hint: "Path to save the agent binary. Specify only directory for default filename, otherwise provide full path with filename.\n\nExample:\n/home/kali\n/home/kali/myagent.exe",
	}
	servers = FormVal[string]{
		Hint: "Server list, one per line - they are tried sequentially until a successfull connection.\n\nExample:\n1.3.3.7:11601\n7.3.3.1:1234\n8.0.0.85:1337",
	}
	proxy = FormVal[string]{
		Hint: "Will override environment variables configuration. Supported protocols: socks5, socks5h, http, https.\n\nExample:\nhttp://proxy:8080\nsocks5://1.2.3.4:3128",
	}
	ignoreEnvProxy = FormVal[bool]{
		Hint: "If checked, will ignore proxies configured via environment variables and attempt a direct connection.",
	}
	goos = FormVal[FormSelectVal]{
		Hint: "Target operating system",
	}
	goarch = FormVal[FormSelectVal]{
		Hint: "Target architecture",
	}
	obfuscate = FormVal[bool]{
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
	saveToField.SetText(saveTo.Last)
	saveToField.SetFocusFunc(func() {
		hintBox.SetText(saveTo.Hint)
	})
	saveToField.SetChangedFunc(func(text string) {
		saveTo.Last = text
	})
	gen.form.AddFormItem(saveToField)

	serversField := tview.NewTextArea()
	serversField.SetLabel("Servers")
	serversField.SetText(servers.Last, true)
	serversField.SetFocusFunc(func() {
		hintBox.SetText(servers.Hint)
	})
	serversField.SetChangedFunc(func() {
		servers.Last = serversField.GetText()
	})
	gen.form.AddFormItem(serversField)

	proxyField := tview.NewInputField()
	proxyField.SetLabel("Proxy")
	proxyField.SetText(proxy.Last)
	proxyField.SetFocusFunc(func() {
		hintBox.SetText(proxy.Hint)
	})
	proxyField.SetChangedFunc(func(text string) {
		proxy.Last = text
	})
	gen.form.AddFormItem(proxyField)

	ignoreEnvProxyField := tview.NewCheckbox()
	ignoreEnvProxyField.SetLabel("Ignore env proxy")
	ignoreEnvProxyField.SetChecked(ignoreEnvProxy.Last)
	ignoreEnvProxyField.SetFocusFunc(func() {
		hintBox.SetText(ignoreEnvProxy.Hint)
	})
	ignoreEnvProxyField.SetChangedFunc(func(checked bool) {
		ignoreEnvProxy.Last = checked
	})
	gen.form.AddFormItem(ignoreEnvProxyField)

	goosField := tview.NewDropDown()
	goosField.SetLabel("OS")
	goosField.SetFocusFunc(func() {
		hintBox.SetText(goos.Hint)
	})
	goosField.SetOptions([]string{"Windows", "Linux", "Darwin"}, func(option string, index int) {
		goos.Last.ID = index
		goos.Last.Value = strings.ToLower(option)
	})
	goosField.SetCurrentOption(goos.Last.ID)
	gen.form.AddFormItem(goosField)

	goarchField := tview.NewDropDown()
	goarchField.SetLabel("Arch")
	goarchField.SetFocusFunc(func() {
		hintBox.SetText(goarch.Hint)
	})
	goarchField.SetOptions([]string{"amd64", "x86", "arm64", "arm"}, func(option string, index int) {
		goarch.Last.ID = index
		goarch.Last.Value = option
	})
	goarchField.SetCurrentOption(goarch.Last.ID)
	gen.form.AddFormItem(goarchField)

	obfuscateField := tview.NewCheckbox()
	obfuscateField.SetLabel("Ignore env proxy")
	obfuscateField.SetChecked(obfuscate.Last)
	obfuscateField.SetFocusFunc(func() {
		hintBox.SetText(obfuscate.Hint)
	})
	obfuscateField.SetBlurFunc(func() {
		hintBox.Clear()
	})
	obfuscateField.SetChangedFunc(func(checked bool) {
		obfuscate.Last = checked
	})
	gen.form.AddFormItem(obfuscateField)

	gen.form.AddButton("Submit", nil)
	gen.form.AddButton("Cancel", nil)

	formFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(gen.form, 0, 2, true).
		AddItem(hintBox, 0, 1, false)

	gen.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(formFlex, 23, 1, true).
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
			saveTo.Last,
			servers.Last,
			goos.Last.Value,
			goarch.Last.Value,
			obfuscate.Last,
			proxy.Last,
			ignoreEnvProxy.Last,
		)
	})
}

func (form *GenerateForm) SetCancelFunc(f func()) {
	btnId := form.form.GetButtonIndex("Cancel")
	submitBtn := form.form.GetButton(btnId)
	submitBtn.SetSelectedFunc(f)
}
