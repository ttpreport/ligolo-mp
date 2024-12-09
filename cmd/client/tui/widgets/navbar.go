package widgets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type NavBar struct {
	*tview.Flex
}

type NavBarElem struct {
	Key  tcell.Key
	Hint string
}

func NewNavBar() *NavBar {
	nav := &NavBar{
		Flex: tview.NewFlex(),
	}

	nav.AddItem(tview.NewBox(), 1, 0, false)

	return nav
}

func NewNavBarElem(k tcell.Key, h string) NavBarElem {
	return NavBarElem{Key: k, Hint: h}
}

func (n *NavBar) AddButton(title string, key tcell.Key) *NavBar {
	button := tview.NewButton(title)

	label := fmt.Sprintf("[yellow][::b]%s[::-][brown] %s", tcell.KeyNames[key], button.GetLabel())
	button.SetLabel(label)

	n.AddItem(button, 0, 1, false)
	n.AddItem(tview.NewBox(), 1, 0, false)

	return n
}

func (n *NavBar) SetData(buttons []NavBarElem) *NavBar {
	n.Clear()
	n.AddItem(tview.NewBox(), 1, 0, false)

	for _, button := range buttons {
		n.AddButton(button.Hint, button.Key)
	}

	n.AddButton("Quit", tcell.KeyCtrlQ)

	return n
}
