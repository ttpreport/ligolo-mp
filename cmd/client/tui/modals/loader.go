package modals

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

type LoaderModal struct {
	tview.Modal
}

func NewLoaderModal() *LoaderModal {
	loader := &LoaderModal{
		Modal: *tview.NewModal(),
	}

	loader.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("loading")))

	return loader
}

func (page *LoaderModal) GetID() string {
	return "loader"
}
