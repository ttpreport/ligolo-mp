package utils

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func EmptyBoxSpace(bgColor tcell.Color) *tview.Box {
	box := tview.NewBox()
	box.SetBackgroundColor(bgColor)
	box.SetBorder(false)

	return box
}
