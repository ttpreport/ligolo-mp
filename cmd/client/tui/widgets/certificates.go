package widgets

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type CertificatesWidget struct {
	*tview.Table
	data []*pb.Cert
}

func NewCertificatesWidget() *CertificatesWidget {
	widget := &CertificatesWidget{
		Table: tview.NewTable(),
	}

	widget.SetSelectable(false, false)
	widget.SetBackgroundColor(style.BgColor)
	widget.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("certificates")))
	widget.SetBorderColor(style.BorderColor)
	widget.SetTitleColor(style.FgColor)
	widget.SetBorder(true)

	widget.SetFocusFunc(func() {
		widget.SetSelectable(true, false)
		widget.ResetSelector()
	})
	widget.SetBlurFunc(func() {
		widget.SetSelectable(false, false)
	})

	return widget
}

func (widget *CertificatesWidget) SetData(data []*pb.Cert) {
	widget.Clear()

	widget.data = data

	widget.Refresh()
	widget.ResetSelector()
}

func (widget *CertificatesWidget) ResetSelector() {
	if len(widget.data) > 0 {
		widget.Select(1, 0) // forcing selection for highlighting to work immediately
	}
}

func (widget *CertificatesWidget) Refresh() {
	headers := []string{"Name", "Expires at"}
	for i := 0; i < len(headers); i++ {
		header := fmt.Sprintf("[::b]%s", strings.ToUpper(headers[i]))
		widget.SetCell(0, i, tview.NewTableCell(header).SetExpansion(1).SetSelectable(false)).SetFixed(1, 0)
	}

	rowId := 1
	for _, elem := range widget.data {
		widget.SetCell(rowId, 0, tview.NewTableCell(elem.Name))
		widget.SetCell(rowId, 1, tview.NewTableCell(elem.ExpiryDate))

		rowId++
	}
}
