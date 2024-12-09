package modals

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
)

type ErrorModal struct {
	tview.Modal
}

func NewErrorModal() *ErrorModal {
	errorPage := &ErrorModal{
		Modal: *tview.NewModal(),
	}
	errorPage.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("error")))
	errorPage.
		SetBackgroundColor(style.ErrorDialogBgColor).
		SetBorderColor(style.ErrorDialogBgColor)
	errorPage.AddButtons([]string{"OK"}).SetButtonBackgroundColor(style.ErrorDialogButtonBgColor)

	return errorPage
}

func (page *ErrorModal) GetID() string {
	return "error"
}
