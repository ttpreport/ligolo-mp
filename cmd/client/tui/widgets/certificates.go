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
	data []*CertificatesWidgetElem
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

	widget.data = nil
	for _, cert := range data {
		widget.data = append(widget.data, NewCertsWidgetElem(cert))
	}

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
		widget.SetCell(rowId, 0, elem.Name())
		widget.SetCell(rowId, 1, elem.ExpiryDate())

		rowId++
	}
}

func (widget *CertificatesWidget) FetchElem(row int) *CertificatesWidgetElem {
	id := max(0, row-1)
	if len(widget.data) > id {
		return widget.data[id]
	}

	return nil
}

func (widget *CertificatesWidget) SetSelectedFunc(f func(*CertificatesWidgetElem)) {
	widget.Table.SetSelectedFunc(func(row, _ int) {
		item := widget.FetchElem(row)
		if item != nil {
			f(item)
		}
	})
}

type CertificatesWidgetElem struct {
	Certificate *pb.Cert
}

func NewCertsWidgetElem(cert *pb.Cert) *CertificatesWidgetElem {
	return &CertificatesWidgetElem{
		Certificate: cert,
	}
}

func (elem *CertificatesWidgetElem) Name() *tview.TableCell {
	return tview.NewTableCell(elem.Certificate.Name)
}

func (elem *CertificatesWidgetElem) ExpiryDate() *tview.TableCell {
	return tview.NewTableCell(elem.Certificate.ExpiryDate)
}
