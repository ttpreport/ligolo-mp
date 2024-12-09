package modals

import (
	"github.com/rivo/tview"
)

type InfoModal struct {
	tview.Modal
}

func NewInfoModal() *ErrorModal {
	errorPage := &ErrorModal{
		Modal: *tview.NewModal(),
	}
	errorPage.AddButtons([]string{"OK"})

	return errorPage
}

func (page *InfoModal) GetID() string {
	return "info"
}
