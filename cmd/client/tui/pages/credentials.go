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

	table *tview.Table

	data []*operator.Operator

	getData    func() ([]*operator.Operator, error)
	newCred    func(path string) error
	deleteCred func(*operator.Operator) error
	connect    func(*operator.Operator) error
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

	creds.initCredentials()

	creds.AddAndSwitchToPage("table", creds.table, true)

	return creds
}

func (creds *CredentialsPage) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		if creds.table.HasFocus() {
			switch key {
			case tcell.KeyCtrlN:
				if creds.newCred != nil {
					filepicker := forms.NewFilepickerForm()
					filepicker.SetSelectFunc(func(path string) {
						err := creds.newCred(path)
						if err != nil {
							creds.ShowError(fmt.Sprintf("Could not add credentials: %s", err), nil)
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
	creds.getData = f
}

func (creds *CredentialsPage) SetNewFunc(f func(path string) error) {
	creds.newCred = f
}

func (creds *CredentialsPage) SetDeleteFunc(f func(*operator.Operator) error) {
	creds.deleteCred = f
}

func (creds *CredentialsPage) SetConnectFunc(f func(*operator.Operator) error) {
	creds.connect = f
}

func (creds *CredentialsPage) initCredentials() {
	creds.table.SetSelectedFunc(func(id, _ int) {
		oper := creds.GetElem(id - 1)

		if oper != nil {
			menu := modals.NewMenuModal(fmt.Sprintf("Credentials — %s", oper.Name))
			cleanup := func() {
				creds.RemovePage(menu.GetID())
			}
			menu.SetCancelFunc(cleanup)

			menu.AddItem(modals.NewMenuModalElem("Connect", func() {
				creds.DoWithLoader("Connecting...", func() {
					err := creds.connect(oper)
					if err != nil {
						creds.ShowError(fmt.Sprintf("Could not connect: %s", err), cleanup)
					}

					cleanup()
				})
			}))

			menu.AddItem(modals.NewMenuModalElem("Remove", func() {
				creds.DoWithConfirm("Are you sure?", func() {
					creds.DoWithLoader("Removing credentials...", func() {
						err := creds.deleteCred(oper)
						if err != nil {
							creds.ShowError(fmt.Sprintf("Could not remove credentials: %s", err), cleanup)
							return
						}
					})
					creds.ShowInfo("Credentials removed", cleanup)
				})
			}))

			creds.AddPage(menu.GetID(), menu, true, true)
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

	if creds.getData == nil {
		return
	}

	data, err := creds.getData()
	if err != nil {
		creds.ShowError(fmt.Sprintf("Could not load credentials: %s", err), nil)
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

func (creds *CredentialsPage) ShowError(text string, done func()) {
	modal := modals.NewErrorModal()
	modal.SetText(text)
	modal.SetDoneFunc(func(_ int, _ string) {
		creds.RemovePage(modal.GetID())

		if done != nil {
			done()
		}
	})
	creds.AddPage(modal.GetID(), modal, true, true)
}

func (creds *CredentialsPage) ShowInfo(text string, done func()) {
	modal := modals.NewInfoModal()
	modal.SetText(text)
	modal.SetDoneFunc(func(_ int, _ string) {
		creds.RemovePage(modal.GetID())

		if done != nil {
			done()
		}
	})
	creds.AddPage(modal.GetID(), modal, true, true)
}
