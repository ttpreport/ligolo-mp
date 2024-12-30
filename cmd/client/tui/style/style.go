package style

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	FgColor     = tcell.ColorFloralWhite
	BgColor     = tview.Styles.PrimitiveBackgroundColor
	BorderColor = tcell.NewRGBColor(135, 135, 175)

	ErrorDialogBgColor       = tcell.NewRGBColor(215, 0, 0)
	ErrorDialogButtonBgColor = tcell.ColorDarkRed

	ModalBgColor = tview.Styles.ContrastBackgroundColor

	HighlightColor = tcell.ColorSteelBlue
)
