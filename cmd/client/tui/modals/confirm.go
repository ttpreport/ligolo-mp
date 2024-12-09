package modals

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

type ConfirmModal struct {
	tview.Modal
}

func NewConfirmModal() *ConfirmModal {
	confirmPage := &ConfirmModal{
		Modal: *tview.NewModal(),
	}
	confirmPage.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("are you sure?")))
	confirmPage.AddButtons([]string{"OK", "Cancel"})

	return confirmPage
}

func (page *ConfirmModal) SetDoneFunc(action func(confirmed bool)) *ConfirmModal {
	page.Modal.SetDoneFunc(func(_ int, label string) {
		if label == "OK" {
			action(true)
		} else {
			action(false)
		}
	})

	return page
}

func (page *ConfirmModal) GetID() string {
	return "confirm"
}
