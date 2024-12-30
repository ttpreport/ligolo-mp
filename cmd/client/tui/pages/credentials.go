package pages

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	forms "github.com/ttpreport/ligolo-mp/cmd/client/tui/forms"
	modals "github.com/ttpreport/ligolo-mp/cmd/client/tui/modals"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/style"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/utils"
	widgets "github.com/ttpreport/ligolo-mp/cmd/client/tui/widgets"
	"github.com/ttpreport/ligolo-mp/internal/operator"
)

type CredentialsPage struct {
	tview.Pages

	table          *tview.Table
	getDataFunc    func() ([]*operator.Operator, error)
	newCredFunc    func(path string) error
	deleteCredFunc func(*operator.Operator) error
	data           []*operator.Operator
}

func NewCredentialsPage() *CredentialsPage {
	creds := &CredentialsPage{
		Pages: *tview.NewPages(),
		table: tview.NewTable(),
	}

	creds.table.SetSelectable(true, false)
	creds.table.SetBackgroundColor(style.BgColor)
	creds.table.SetTitle(fmt.Sprintf("[::b]%s", strings.ToUpper("credentials")))
	creds.table.SetBorderColor(style.BorderColor)
	creds.table.SetTitleColor(style.FgColor)
	creds.table.SetBorder(true)

	creds.AddAndSwitchToPage("table", creds.table, true)

	return creds
}

func (creds *CredentialsPage) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		if creds.table.HasFocus() {
			switch key {
			case tcell.KeyCtrlN:
				if creds.newCredFunc != nil {
					filepicker := forms.NewFilepickerForm()
					filepicker.SetSelectFunc(func(path string) {
						err := creds.newCredFunc(path)
						if err != nil {
							creds.ShowError(fmt.Sprintf("Could not add credentials: %s", err))
							filepicker.Reset()
							return
						}

						creds.RemovePage(filepicker.GetID())
						creds.RefreshData()
					})
					filepicker.SetCancelFunc(func() {
						creds.RemovePage(filepicker.GetID())
					})
					creds.AddPage(filepicker.GetID(), filepicker, true, true)
				}
			case tcell.KeyCtrlD:
				if creds.deleteCredFunc != nil {
					id, _ := creds.table.GetSelection()
					oper := creds.GetElem(id - 1)

					creds.DoWithConfirm("Are you sure?", func() {
						err := creds.deleteCredFunc(oper)
						if err != nil {
							creds.ShowError(fmt.Sprintf("Could not delete credentials: %s", err))
						}
					})
				}
			default:
				defaultHandler := creds.Pages.InputHandler()
				defaultHandler(event, setFocus)
			}
		} else {
			switch key {
			case tcell.KeyEscape:
				if creds.GetPageCount() > 1 {
					frontPage, _ := creds.GetFrontPage()
					creds.RemovePage(frontPage)
				}
			default:
				defaultHandler := creds.Pages.InputHandler()
				defaultHandler(event, setFocus)
			}
		}
	}
}

func (creds *CredentialsPage) GetID() string {
	return "credentials"
}

func (creds *CredentialsPage) GetElem(id int) *operator.Operator {
	if id < len(creds.data) {
		return creds.data[id]
	}

	return nil
}

func (creds *CredentialsPage) SetDataFunc(f func() ([]*operator.Operator, error)) {
	creds.getDataFunc = f
}

func (creds *CredentialsPage) SetNewFunc(f func(path string) error) {
	creds.newCredFunc = f
}

func (creds *CredentialsPage) SetDeleteFunc(f func(*operator.Operator) error) {
	creds.deleteCredFunc = f
}

func (creds *CredentialsPage) SetSelectedFunc(f func(*operator.Operator) error) {
	creds.table.SetSelectedFunc(func(id, _ int) {
		oper := creds.GetElem(id - 1)

		if oper != nil {
			creds.DoWithLoader("Connecting...", func() {
				err := f(oper)
				if err != nil {
					creds.ShowError(fmt.Sprintf("Could not connect: %s", err))
				}
			})
		}
	})
}

func (creds *CredentialsPage) RefreshData() {
	creds.table.Clear()

	headers := []string{"Login", "Server", "Is Admin"}
	for i := 0; i < len(headers); i++ {
		header := fmt.Sprintf("[::b]%s", strings.ToUpper(headers[i]))
		creds.table.SetCell(0, i, tview.NewTableCell(header).SetExpansion(1).SetSelectable(false)).SetFixed(1, 0)
	}

	if creds.getDataFunc == nil {
		return
	}

	data, err := creds.getDataFunc()
	if err != nil {
		creds.ShowError(fmt.Sprintf("Could not load credentials: %s", err))
		return
	}
	creds.data = data

	for i := 0; i < len(creds.data); i++ {
		rowIdx := i + 1
		name := creds.data[i].Name
		server := creds.data[i].Server
		isAdmin := utils.HumanBool(creds.data[i].IsAdmin)

		creds.table.SetCell(rowIdx, 0, tview.NewTableCell(name))
		creds.table.SetCell(rowIdx, 1, tview.NewTableCell(server))
		creds.table.SetCell(rowIdx, 2, tview.NewTableCell(isAdmin))
	}
}

func (creds *CredentialsPage) GetNavBar() []widgets.NavBarElem {
	return []widgets.NavBarElem{
		widgets.NewNavBarElem(tcell.KeyEnter, "Select"),
		widgets.NewNavBarElem(tcell.KeyCtrlN, "Add new"),
		widgets.NewNavBarElem(tcell.KeyCtrlD, "Delete"),
	}
}

func (creds *CredentialsPage) DoWithConfirm(text string, action func()) {
	modal := modals.NewConfirmModal()
	modal.SetText(text)
	modal.SetDoneFunc(func(confirmed bool) {
		if confirmed {
			action()
		}

		creds.RemovePage(modal.GetID())
	})
	creds.AddPage(modal.GetID(), modal, true, true)
}

func (creds *CredentialsPage) DoWithLoader(text string, action func()) {
	go func() {
		modal := modals.NewLoaderModal()
		modal.SetText(text)
		creds.AddPage(modal.GetID(), modal, true, true)
		action()
		creds.RemovePage(modal.GetID())
	}()
}

func (creds *CredentialsPage) ShowError(text string) {
	modal := modals.NewErrorModal()
	modal.SetText(text)
	modal.SetDoneFunc(func(_ int, _ string) {
		creds.RemovePage(modal.GetID())
	})
	creds.AddPage(modal.GetID(), modal, true, true)
}

func (dash *CredentialsPage) ShowInfo(text string) {
	modal := modals.NewInfoModal()
	modal.SetText(text)
	modal.SetDoneFunc(func(_ int, _ string) {
		dash.RemovePage(modal.GetID())
	})
	dash.AddPage(modal.GetID(), modal, true, true)
}
