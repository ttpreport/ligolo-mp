package widgets

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
)

type LogsWidget struct {
	tview.TextView
}

func NewLogsWidget() *LogsWidget {
	widget := &LogsWidget{
		TextView: *tview.NewTextView(),
	}

	widget.SetTextAlign(tview.AlignTop).SetDynamicColors(true)
	widget.SetBorder(true)
	widget.SetBorderColor(style.BorderColor)
	widget.SetTitleColor(style.FgColor)
	widget.SetBackgroundColor(style.BgColor)
	widget.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("log")))

	return widget
}

func (widget *LogsWidget) Write(p []byte) (n int, err error) {
	currentText := widget.GetText(false)
	newText := fmt.Sprintf("%s%s", string(p), currentText)
	widget.SetText(newText)
	return len(p), nil
}

func (widget *LogsWidget) Truncate() {
	widget.SetText("")
}
