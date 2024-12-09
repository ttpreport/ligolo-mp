package pages

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	widgets "github.com/ttpreport/ligolo-mp/cmd/client/tui/widgets"
)

type AdminPage struct {
	tview.Pages

	operatorFunc func()
}

func NewAdminPage() *AdminPage {
	admin := &AdminPage{
		Pages: *tview.NewPages(),
	}

	return admin
}

func (admin *AdminPage) GetID() string {
	return "admin"
}

func (admin *AdminPage) GetNavBar() []widgets.NavBarElem {
	return []widgets.NavBarElem{
		widgets.NewNavBarElem(tcell.KeyCtrlA, "Operator"),
	}
}

func (admin *AdminPage) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()
		switch key {
		case tcell.KeyCtrlA:
			admin.operatorFunc()
		default:
			defaultHandler := admin.Pages.InputHandler()
			defaultHandler(event, setFocus)
		}

	}
}

func (admin *AdminPage) SetOperatorFunc(f func()) {
	admin.operatorFunc = f
}
