package widgets

import (
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
	"github.com/ttpreport/ligolo-mp/internal/events"
)

type LogsWidget struct {
	tview.TextView
}

func NewLogsWidget() *LogsWidget {
	widget := &LogsWidget{
		TextView: *tview.NewTextView(),
	}

	widget.SetTextAlign(tview.AlignTop).SetTextColor(style.FgColor).SetDynamicColors(true)
	widget.SetBorder(true)
	widget.SetBorderColor(style.BorderColor)
	widget.SetTitleColor(style.FgColor)
	widget.SetBackgroundColor(style.BgColor)
	widget.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("log")))

	return widget
}

func (widget *LogsWidget) Append(severity events.EventType, data string) {
	currentLog := widget.GetText(true)
	date := time.Now().UTC().Format(time.RFC3339)
	newLog := fmt.Sprintf("%s (%s) | %s\n%s", date, severity, data, currentLog)
	widget.SetText(newLog)
}

func (widget *LogsWidget) Truncate() {
	widget.SetText("")
}
