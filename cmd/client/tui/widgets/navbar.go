package widgets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type NavBar struct {
	*tview.Flex
	clickHandler func(tcell.Key)
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

func (n *NavBar) SetClickHandler(handler func(tcell.Key)) {
	n.clickHandler = handler
}

func (n *NavBar) AddButton(title string, key tcell.Key) *NavBar {
	button := tview.NewButton(title)

	label := fmt.Sprintf("[yellow][::b]%s[::-][brown] %s", tcell.KeyNames[key], button.GetLabel())
	button.SetLabel(label)

	// Add mouse support - when button is clicked, call the click handler
	button.SetSelectedFunc(func() {
		if n.clickHandler != nil {
			n.clickHandler(key)
		}
	})

	n.AddItem(button, 0, 1, true)
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
