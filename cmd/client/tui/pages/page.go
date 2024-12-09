package pages

import (
	"github.com/rivo/tview"
	widgets "github.com/ttpreport/ligolo-mp/cmd/client/tui/widgets"
)

type Page interface {
	tview.Primitive
	GetID() string
	GetNavBar() []widgets.NavBarElem
}
