package pages

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/forms"
	route_forms "github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/forms/route"
	modals "github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/modals"
	widgets "github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/widgets"
	"github.com/ttpreport/ligolo-mp/v2/internal/config"
	"github.com/ttpreport/ligolo-mp/v2/internal/operator"
	"github.com/ttpreport/ligolo-mp/v2/internal/session"
)

type DashboardPage struct {
	tview.Pages

	app         *tview.Application
	setFocus    func(tview.Primitive)
	flex        *tview.Flex
	server      *widgets.ServerWidget
	sessions    *widgets.SessionsWidget
	interfaces  *widgets.InterfacesWidget
	routes      *widgets.RoutesWidget
	redirectors *widgets.RedirectorsWidget

	data []*session.Session

	fetchData                   func() ([]*session.Session, error)
	getMetadata                 func() (*config.Config, *operator.Operator, error)
	adminFunc                   func()
	generateFunc                func(path string, servers string, goos string, goarch string, obfuscate bool, proxy string, ignoreEnvProxy bool) (string, error)
	sessionStartFunc            func(*session.Session) error
	sessionStopFunc             func(*session.Session) error
	sessionRenameFunc           func(*session.Session, string) error
	sessionAddRouteFunc         func(*session.Session, string, int, bool) error
	sessionEditRouteFunc        func(*session.Session, string, string, int, bool) error
	sessionMoveRouteFunc        func(*session.Session, string, string) error
	sessionRemoveRouteFunc      func(*session.Session, string) error
	sessionAddRedirectorFunc    func(*session.Session, string, string, string) error
	sessionRemoveRedirectorFunc func(*session.Session, string) error
	sessionRemoveFunc           func(*session.Session) error
	tracerouteFunc              func(string) ([]string, error)

	operator *operator.Operator
}

func NewDashboardPage() *DashboardPage {
	dash := &DashboardPage{
		Pages:       *tview.NewPages(),
		flex:        tview.NewFlex(),
		server:      widgets.NewServerWidget(),
		sessions:    widgets.NewSessionsWidget(),
		interfaces:  widgets.NewInterfacesWidget(),
		routes:      widgets.NewRoutesWidget(),
		redirectors: widgets.NewRedirectorsWidget(),
	}

	dash.initSessionsWidget()
	dash.initRoutesWidget()
	dash.initRedirectorsWidget()

	firstRow := tview.NewFlex()
	firstRow.SetDirection(tview.FlexColumn)
	firstRow.AddItem(dash.sessions, 0, 65, true)
	firstRow.AddItem(dash.interfaces, 0, 35, false)

	secondRow := tview.NewFlex()
	secondRow.SetDirection(tview.FlexColumn)
	secondRow.AddItem(dash.routes, 0, 50, false)
	secondRow.AddItem(dash.redirectors, 0, 50, false)

	dash.flex.SetDirection(tview.FlexRow)
	dash.flex.AddItem(dash.server, 3, 0, false)
	dash.flex.AddItem(firstRow, 0, 50, true)
	dash.flex.AddItem(secondRow, 0, 50, false)

	dash.Reset()

	return dash
}

func (dash *DashboardPage) Reset() {
	for _, page := range dash.GetPageNames(false) {
		dash.RemovePage(page)
	}

	dash.AddAndSwitchToPage("main", dash.flex, true)
}

func (dash *DashboardPage) initSessionsWidget() {
	dash.sessions.SetSelectionChangedFunc(func(sess *session.Session) {
		dash.sessions.SetSelectedSession(sess)
		dash.interfaces.SetSelectedSession(sess)
		dash.routes.SetSelectedSession(sess)
		dash.redirectors.SetSelectedSession(sess)
	})

	dash.sessions.SetSelectedFunc(func(sess *session.Session) {
		name := sess.GetName()
		menu := modals.NewMenuModal(fmt.Sprintf("Session — %s", name))
		cleanup := func() {
			dash.RemovePage(menu.GetID())
			dash.setFocus(dash.sessions)
		}

		if sess.IsRelaying {
			menu.AddItem(modals.NewMenuModalElem("Stop relay", func() {
				dash.DoWithLoader("Stopping relay...", func() {
					err := dash.sessionStopFunc(sess)
					if err != nil {
						dash.ShowError(fmt.Sprintf("Could not stop relay: %s", err), cleanup)
						return
					}

					dash.ShowInfo("Relay stopped", cleanup)
				})
			}))
		} else {
			menu.AddItem(modals.NewMenuModalElem("Start relay", func() {
				dash.DoWithLoader("Starting relay...", func() {
					err := dash.sessionStartFunc(sess)
					if err != nil {
						dash.ShowError(fmt.Sprintf("Could not start relay: %s", err), cleanup)
						return
					}

					dash.ShowInfo("Relay started", cleanup)
				})
			}))
		}

		menu.AddItem(modals.NewMenuModalElem("Rename", func() {
			ren := forms.NewRenameForm()
			ren.SetSubmitFunc(func(alias string) {
				dash.DoWithLoader("Renaming session...", func() {
					err := dash.sessionRenameFunc(sess, alias)
					if err != nil {
						dash.ShowError(fmt.Sprintf("Could not rename session: %s", err), cleanup)
						return
					}

					dash.RemovePage(ren.GetID())
					dash.ShowInfo("Session renamed", cleanup)
				})
			})
			ren.SetCancelFunc(func() {
				dash.RemovePage(ren.GetID())
				cleanup()
			})
			dash.AddPage(ren.GetID(), ren, true, true)
		}))

		menu.AddItem(modals.NewMenuModalElem("Add route", func() {
			route := route_forms.NewAddRouteForm()
			route.SetSubmitFunc(func(cidr string, metric int, loopback bool) {
				dash.DoWithLoader("Adding route...", func() {
					err := dash.sessionAddRouteFunc(sess, cidr, metric, loopback)
					if err != nil {
						dash.app.QueueUpdateDraw(func() { dash.RemovePage(route.GetID()) })
						dash.ShowError(fmt.Sprintf("Could not add route: %s", err), cleanup)
						return
					}

					dash.app.QueueUpdateDraw(func() { dash.RemovePage(route.GetID()) })
					dash.ShowInfo("Route added", cleanup)
				})
			})
			route.SetCancelFunc(func() {
				dash.RemovePage(route.GetID())
				dash.setFocus(dash.sessions)
				cleanup()
			})
			dash.AddPage(route.GetID(), route, true, true)
		}))

		menu.AddItem(modals.NewMenuModalElem("Add redirector", func() {
			redir := forms.NewAddRedirectorForm()
			redir.SetSubmitFunc(func(from string, to string, proto string) {
				dash.DoWithLoader("Adding redirector...", func() {
					err := dash.sessionAddRedirectorFunc(sess, from, to, proto)
					if err != nil {
						dash.app.QueueUpdateDraw(func() { dash.RemovePage(redir.GetID()) })
						dash.ShowError(fmt.Sprintf("Could not add route: %s", err), cleanup)
						return
					}

					dash.app.QueueUpdateDraw(func() { dash.RemovePage(redir.GetID()) })
					dash.ShowInfo("Redirector added", cleanup)
				})
			})
			redir.SetCancelFunc(func() {
				dash.RemovePage(redir.GetID())
				dash.setFocus(dash.sessions)
				cleanup()
			})
			dash.AddPage(redir.GetID(), redir, true, true)
		}))

		menu.AddItem(modals.NewMenuModalElem("Remove", func() {
			dash.DoWithConfirm("Are you sure?", func() {
				dash.DoWithLoader("Removing session...", func() {
					err := dash.sessionRemoveFunc(sess)
					if err != nil {
						dash.ShowError(fmt.Sprintf("Could not remove session: %s", err), cleanup)
						return
					}

					dash.ShowInfo("Session removed", cleanup)
				})
			})
		}))

		menu.SetCancelFunc(cleanup)

		dash.AddPage(menu.GetID(), menu, true, true)
	})
}

func (dash *DashboardPage) initRoutesWidget() {
	dash.routes.SetSelectionChangedFunc(func(elem *widgets.RoutesWidgetElem) {
		dash.sessions.SetSelectedSession(elem.Session)
		dash.interfaces.SetSelectedSession(elem.Session)
		dash.routes.SetSelectedSession(elem.Session)
		dash.redirectors.SetSelectedSession(elem.Session)
	})

	dash.routes.SetSelectedFunc(func(elem *widgets.RoutesWidgetElem) {
		menu := modals.NewMenuModal("Route")
		cleanup := func() {
			dash.RemovePage(menu.GetID())
			dash.setFocus(dash.routes)
		}

		menu.AddItem(modals.NewMenuModalElem("Edit", func() {
			routeEdit := route_forms.NewEditRouteForm(elem.Route)
			routeEdit.SetSubmitFunc(func(cidr string, metric int, loopback bool) {
				dash.DoWithLoader("Editing route...", func() {
					err := dash.sessionEditRouteFunc(elem.Session, elem.Route.ID, cidr, metric, loopback)
					if err != nil {
						dash.app.QueueUpdateDraw(func() { dash.RemovePage(routeEdit.GetID()) })
						dash.ShowError(fmt.Sprintf("Could not edit route: %s", err), cleanup)
						return
					}

					dash.app.QueueUpdateDraw(func() { dash.RemovePage(routeEdit.GetID()) })
					dash.ShowInfo("Route edited", cleanup)
				})
			})
			routeEdit.SetCancelFunc(func() {
				dash.RemovePage(routeEdit.GetID())
				cleanup()
			})
			dash.AddPage(routeEdit.GetID(), routeEdit, true, true)
		}))

		menu.AddItem(modals.NewMenuModalElem("Move", func() {
			sessions := dash.GetData()
			routeMove := route_forms.NewMoveRouteForm(sessions)
			routeMove.SetSubmitFunc(func(targetSessionID string) {
				dash.DoWithLoader("Moving route...", func() {
					err := dash.sessionMoveRouteFunc(elem.Session, elem.Route.ID, targetSessionID)
					if err != nil {
						dash.app.QueueUpdateDraw(func() { dash.RemovePage(routeMove.GetID()) })
						dash.ShowError(fmt.Sprintf("Could not move route: %s", err), cleanup)
						return
					}

					dash.app.QueueUpdateDraw(func() { dash.RemovePage(routeMove.GetID()) })
					dash.ShowInfo("Route moved", cleanup)
				})
			})
			routeMove.SetCancelFunc(func() {
				dash.RemovePage(routeMove.GetID())
				cleanup()
			})
			dash.AddPage(routeMove.GetID(), routeMove, true, true)
		}))

		menu.AddItem(modals.NewMenuModalElem("Remove", func() {
			dash.DoWithConfirm("Are you sure?", func() {
				dash.DoWithLoader("Removing route...", func() {
					err := dash.sessionRemoveRouteFunc(elem.Session, elem.Route.ID)
					if err != nil {
						dash.ShowError(fmt.Sprintf("Could not remove route: %s", err), cleanup)
						return
					}

					dash.ShowInfo("Route removed", cleanup)
				})
			})
		}))

		menu.SetCancelFunc(cleanup)

		dash.AddPage(menu.GetID(), menu, true, true)
	})
}

func (dash *DashboardPage) initRedirectorsWidget() {
	dash.redirectors.SetSelectionChangedFunc(func(elem *widgets.RedirectorsWidgetElem) {
		dash.sessions.SetSelectedSession(elem.Session)
		dash.interfaces.SetSelectedSession(elem.Session)
		dash.routes.SetSelectedSession(elem.Session)
		dash.redirectors.SetSelectedSession(elem.Session)
	})

	dash.redirectors.SetSelectedFunc(func(elem *widgets.RedirectorsWidgetElem) {
		menu := modals.NewMenuModal("Redirector")
		cleanup := func() {
			dash.RemovePage(menu.GetID())
			dash.setFocus(dash.redirectors)
		}

		menu.AddItem(modals.NewMenuModalElem("Remove", func() {
			dash.DoWithLoader("Removing route...", func() {
				err := dash.sessionRemoveRedirectorFunc(elem.Session, elem.Redirector.ID)
				if err != nil {
					dash.ShowError(fmt.Sprintf("Could not remove redirector: %s", err), cleanup)
					return
				}

				dash.ShowInfo("Redirector removed", cleanup)
			})
		}))

		menu.SetCancelFunc(cleanup)

		dash.AddPage(menu.GetID(), menu, true, true)
	})
}

func (dash *DashboardPage) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		if dash.flex.HasFocus() {
			switch key {
			case tcell.KeyCtrlA:
				if dash.operator.IsAdmin {
					dash.adminFunc()
				}
			case tcell.KeyTab:
				focusOrder := []tview.Primitive{
					dash.sessions,
					dash.interfaces,
					dash.routes,
					dash.redirectors,
				}

				for id, pane := range focusOrder {
					if pane.HasFocus() {
						nextId := (id + 1) % len(focusOrder)
						setFocus(focusOrder[nextId])
						break
					}
				}
			case tcell.KeyCtrlN:
				gen := forms.NewGenerateForm()
				gen.SetSubmitFunc(func(path string, servers string, goos string, goarch string, obfuscate bool, proxy string, ignoreEnvProxy bool) {
					dash.DoWithLoader("Generating agent...", func() {
						fullPath, err := dash.generateFunc(path, servers, goos, goarch, obfuscate, proxy, ignoreEnvProxy)
						if err != nil {
							dash.ShowError(fmt.Sprintf("Could not generate agent: %s", err), nil)
							return
						}

						dash.app.QueueUpdateDraw(func() { dash.RemovePage(gen.GetID()) })
						dash.ShowInfo(fmt.Sprintf("Agent binary saved to %s", fullPath), nil)
					})
				})
				gen.SetCancelFunc(func() {
					dash.RemovePage(gen.GetID())
				})
				dash.AddPage(gen.GetID(), gen, true, true)
			case tcell.KeyCtrlT:
				trace := forms.NewTracerouteForm()
				trace.SetSubmitFunc(func(address string) {
					dash.DoWithLoader("Tracing route...", func() {
						routes, err := dash.tracerouteFunc(address)
						if err != nil {
							dash.ShowError(fmt.Sprintf("Could not trace route: %s", err), nil)
							return
						}

						dash.app.QueueUpdateDraw(func() { dash.RemovePage(trace.GetID()) })
						dash.ShowText(fmt.Sprintf("Tracing %s", address), strings.Join(routes, "\n"), nil)
					})
				})
				trace.SetCancelFunc(func() {
					dash.RemovePage(trace.GetID())
				})
				dash.AddPage(trace.GetID(), trace, true, true)
			default:
				defaultHandler := dash.flex.InputHandler()
				defaultHandler(event, setFocus)
			}
		} else {
			switch key {
			case tcell.KeyEscape:
				if dash.GetPageCount() > 1 {
					frontPage, _ := dash.GetFrontPage()
					dash.RemovePage(frontPage)
				}
			default:
				defaultHandler := dash.Pages.InputHandler()
				defaultHandler(event, setFocus)
			}
		}
	}
}

func (dash *DashboardPage) Focus(delegate func(p tview.Primitive)) {
	dash.setFocus = delegate
	dash.Pages.Focus(delegate)
}

func (dash *DashboardPage) GetID() string {
	return "dashboard"
}

func (dash *DashboardPage) SetApp(app *tview.Application) {
	dash.app = app
}

func (dash *DashboardPage) SetOperator(oper *operator.Operator) {
	dash.operator = oper
}

func (dash *DashboardPage) SetAdminFunc(f func()) {
	dash.adminFunc = f
}

func (dash *DashboardPage) SetDataFunc(f func() ([]*session.Session, error)) {
	dash.fetchData = f
}

func (dash *DashboardPage) SetMetadataFunc(f func() (*config.Config, *operator.Operator, error)) {
	dash.getMetadata = f
}

func (dash *DashboardPage) SetGenerateFunc(f func(string, string, string, string, bool, string, bool) (string, error)) {
	dash.generateFunc = f
}

func (dash *DashboardPage) SetSessionStartFunc(f func(*session.Session) error) {
	dash.sessionStartFunc = f
}

func (dash *DashboardPage) SetSessionStopFunc(f func(*session.Session) error) {
	dash.sessionStopFunc = f
}

func (dash *DashboardPage) SetSessionRenameFunc(f func(*session.Session, string) error) {
	dash.sessionRenameFunc = f
}

func (dash *DashboardPage) SetSessionAddRouteFunc(f func(*session.Session, string, int, bool) error) {
	dash.sessionAddRouteFunc = f
}

func (dash *DashboardPage) SetSessionEditRouteFunc(f func(*session.Session, string, string, int, bool) error) {
	dash.sessionEditRouteFunc = f
}

func (dash *DashboardPage) SetSessionMoveRouteFunc(f func(*session.Session, string, string) error) {
	dash.sessionMoveRouteFunc = f
}

func (dash *DashboardPage) SetSessionRemoveRouteFunc(f func(*session.Session, string) error) {
	dash.sessionRemoveRouteFunc = f
}

func (dash *DashboardPage) SetSessionAddRedirectorFunc(f func(*session.Session, string, string, string) error) {
	dash.sessionAddRedirectorFunc = f
}

func (dash *DashboardPage) SetSessionRemoveRedirectorFunc(f func(*session.Session, string) error) {
	dash.sessionRemoveRedirectorFunc = f
}

func (dash *DashboardPage) SetSessionKillFunc(f func(*session.Session) error) {
	dash.sessionRemoveFunc = f
}

func (dash *DashboardPage) SetTracerouteFunc(f func(string) ([]string, error)) {
	dash.tracerouteFunc = f
}

func (dash *DashboardPage) RefreshData() {
	data, err := dash.fetchData()
	if err != nil {
		dash.app.QueueUpdateDraw(func() {
			dash.ShowError(fmt.Sprintf("Could not fetch data: %s", err), nil)
		})
		return
	}

	cfg, oper, err := dash.getMetadata()
	if err != nil {
		dash.app.QueueUpdateDraw(func() {
			dash.ShowError(fmt.Sprintf("Could not fetch metadata: %s", err), nil)
		})
		return
	}

	dash.app.QueueUpdateDraw(func() {
		dash.data = data
		dash.sessions.SetData(data)
		dash.interfaces.SetData(data)
		dash.routes.SetData(data)
		dash.redirectors.SetData(data)
		dash.server.SetData(cfg, oper)
	})
}

func (dash *DashboardPage) GetData() []*session.Session {
	return dash.data
}

func (dash *DashboardPage) GetNavBar() []widgets.NavBarElem {
	navbar := []widgets.NavBarElem{
		widgets.NewNavBarElem(tcell.KeyCtrlN, "Generate"),
		widgets.NewNavBarElem(tcell.KeyCtrlT, "Traceroute"),
		widgets.NewNavBarElem(tcell.KeyTab, "Switch pane"),
	}

	if dash.operator.IsAdmin {
		navbar = append([]widgets.NavBarElem{
			widgets.NewNavBarElem(tcell.KeyCtrlA, "Admin")},
			navbar...,
		)
	}

	return navbar
}

func (dash *DashboardPage) DoWithLoader(text string, action func()) {
	go func() {
		modal := modals.NewLoaderModal()
		modal.SetText(text)
		dash.app.QueueUpdateDraw(func() {
			dash.AddPage(modal.GetID(), modal, true, true)
		})
		action()
		dash.app.QueueUpdateDraw(func() {
			dash.RemovePage(modal.GetID())
		})
	}()
}

func (dash *DashboardPage) DoWithConfirm(text string, action func()) {
	modal := modals.NewConfirmModal()
	modal.SetText(text)
	modal.SetDoneFunc(func(confirmed bool) {
		if confirmed {
			action()
		}

		dash.RemovePage(modal.GetID())
	})
	dash.AddPage(modal.GetID(), modal, true, true)
}

func (dash *DashboardPage) ShowError(text string, done func()) {
	dash.app.QueueUpdateDraw(func() {
		modal := modals.NewErrorModal()
		modal.SetText(text)
		modal.SetDoneFunc(func(_ int, _ string) {
			dash.RemovePage(modal.GetID())

			if done != nil {
				done()
			}
		})
		dash.AddPage(modal.GetID(), modal, true, true)
	})
}

func (dash *DashboardPage) ShowInfo(text string, done func()) {
	dash.app.QueueUpdateDraw(func() {
		modal := modals.NewInfoModal()
		modal.SetText(text)
		modal.SetDoneFunc(func(_ int, _ string) {
			dash.RemovePage(modal.GetID())

			if done != nil {
				done()
			}
		})
		dash.AddPage(modal.GetID(), modal, true, true)
	})
}

func (dash *DashboardPage) ShowText(title string, text string, done func()) {
	dash.app.QueueUpdateDraw(func() {
		modal := modals.NewTextModal(title, text)
		modal.SetDoneFunc(func() {
			dash.RemovePage(modal.GetID())
		})
		dash.AddPage(modal.GetID(), modal, true, true)
	})
}
