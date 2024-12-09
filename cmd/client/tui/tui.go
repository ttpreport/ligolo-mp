package tui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	modals "github.com/ttpreport/ligolo-mp/cmd/client/tui/modals"
	pages "github.com/ttpreport/ligolo-mp/cmd/client/tui/pages"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui/utils"
	widgets "github.com/ttpreport/ligolo-mp/cmd/client/tui/widgets"
	"github.com/ttpreport/ligolo-mp/internal/events"
	"github.com/ttpreport/ligolo-mp/internal/operator"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type App struct {
	*tview.Application
	pages        *tview.Pages
	credentials  *pages.CredentialsPage
	dashboard    *pages.DashboardPage
	admin        *pages.AdminPage
	confirmModal *modals.ConfirmModal
	loaderModal  *modals.LoaderModal
	navbar       *widgets.NavBar
	operService  *operator.OperatorService
	operator     *operator.Operator
	currentPage  string
}

func NewApp(operService *operator.OperatorService) *App {
	app := &App{
		Application: tview.NewApplication(),
		pages:       tview.NewPages(),
		credentials: pages.NewCredentialsPage(),
		dashboard:   pages.NewDashboardPage(),
		admin:       pages.NewAdminPage(),
		navbar:      widgets.NewNavBar(),
		operService: operService,
	}

	app.initCredentials()
	app.initDashboard()
	app.initAdmin()

	app.SwitchToPage(app.credentials)

	go app.refresh()

	return app
}

func (app *App) refresh() {
	tick := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-tick.C:
			app.Draw()
		}
	}
}

func (app *App) SwitchToPage(p pages.Page) {
	app.pages.RemovePage(app.currentPage)

	app.currentPage = p.GetID()
	app.pages.AddAndSwitchToPage(app.currentPage, p, true)
	app.navbar.SetData(p.GetNavBar())
}

func (app *App) initCredentials() {
	app.credentials.SetDataFunc(func() ([]*operator.Operator, error) {
		return app.operService.AllOperators()
	})

	app.credentials.SetSelectedFunc(func(oper *operator.Operator) error {
		err := app.SwitchOperator(oper)
		if err != nil {
			return err
		}

		app.dashboard.SetOperator(oper)
		app.SwitchToPage(app.dashboard)
		return nil
	})

	app.credentials.SetDeleteFunc(func(oper *operator.Operator) error {
		if oper != nil {
			_, err := app.operService.RemoveOperator(oper.Name)
			if err != nil {
				return err
			}

			app.credentials.RefreshData()
		}

		return nil
	})

	app.credentials.SetNewFunc(func(path string) error {
		_, err := app.operService.NewOperatorFromFile(path)
		if err != nil {
			return err
		}

		return nil
	})

	app.pages.AddPage(app.credentials.GetID(), app.credentials, true, false)
}

func (app *App) initDashboard() {
	app.dashboard.SetAdminFunc(func() {
		app.SwitchToPage(app.admin)
	})

	app.dashboard.SetDataFunc(func() ([]*pb.Session, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()

		r, err := app.operator.Client().GetSessions(ctx, &pb.Empty{})
		if err != nil {
			return nil, err
		}

		return r.Sessions, nil
	})

	app.dashboard.SetDisconnectFunc(func() {
		app.operator.Disconnect()
		app.operator = nil

		app.SwitchToPage(app.credentials)
	})

	app.dashboard.SetGenerateFunc(func(path string, req *pb.GenerateAgentReq) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		r, err := app.operator.Client().GenerateAgent(ctx, req)
		if err != nil {
			return "", err
		}

		if err = os.WriteFile(path, r.AgentBinary, 0755); err != nil {
			return "", err
		}

		return filepath.Abs(path)
	})

	app.dashboard.SetSessionStartFunc(func(sess *pb.Session) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		_, err := app.operator.Client().StartRelay(ctx, &pb.StartRelayReq{
			SessionID: sess.ID,
		})
		return err
	})

	app.dashboard.SetSessionStopFunc(func(sess *pb.Session) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		_, err := app.operator.Client().StopRelay(ctx, &pb.StopRelayReq{
			SessionID: sess.ID,
		})
		return err
	})

	app.dashboard.SetSessionRenameFunc(func(sess *pb.Session, alias string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		_, err := app.operator.Client().RenameSession(ctx, &pb.RenameSessionReq{
			SessionID: sess.ID,
			Alias:     alias,
		})
		return err
	})

	app.dashboard.SetSessionAddRouteFunc(func(sess *pb.Session, cidr string, loopback bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		_, err := app.operator.Client().AddRoute(ctx, &pb.AddRouteReq{
			SessionID: sess.ID,
			Route: &pb.Route{
				Cidr:       cidr,
				IsLoopback: loopback,
			},
		})
		return err
	})

	app.dashboard.SetSessionRemoveRouteFunc(func(sess *pb.Session, cidr string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		_, err := app.operator.Client().DelRoute(ctx, &pb.DelRouteReq{
			SessionID: sess.ID,
			Cidr:      cidr,
		})
		return err
	})

	app.dashboard.SetSessionAddRedirectorFunc(func(sess *pb.Session, from string, to string, proto string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		_, err := app.operator.Client().AddRedirector(ctx, &pb.AddRedirectorReq{
			SessionID: sess.ID,
			From:      from,
			To:        to,
			Protocol:  proto,
		})
		return err
	})

	app.dashboard.SetSessionRemoveRedirectorFunc(func(sess *pb.Session, redirectorID string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		_, err := app.operator.Client().DelRedirector(ctx, &pb.DelRedirectorReq{
			SessionID:    sess.ID,
			RedirectorID: redirectorID,
		})
		return err
	})

	app.dashboard.SetSessionKillFunc(func(sess *pb.Session) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()
		_, err := app.operator.Client().KillSession(ctx, &pb.KillSessionReq{
			SessionID: sess.ID,
		})
		return err
	})
}

func (app *App) initAdmin() {
	app.admin.SetOperatorFunc(func() {
		app.SwitchToPage(app.dashboard)
	})

	app.pages.AddPage(app.credentials.GetID(), app.credentials, true, false)
}

func (app *App) Run() error {
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(app.pages, 0, 99, true).
		AddItem(app.navbar, 0, 1, false)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch key := event.Key(); key {
		case utils.AppInterruptKey.Key:
			return tcell.NewEventKey(key, 0, tcell.ModNone)

		case utils.AppExitKey.Key:
			app.Stop()
			os.Exit(0)
			return nil
		}

		return event
	})

	app.credentials.RefreshData()

	if err := app.SetRoot(flex, true).SetFocus(app.pages).EnablePaste(true).Run(); err != nil {
		return err
	}

	return nil
}

func (app *App) log(severity events.EventType, data string) {
	app.dashboard.AppendLog(severity, data)

}

func (app *App) HandleOperatorEvents() {
	for {
		var eventStream pb.Ligolo_JoinClient
		var err error

		for {
			if app.operator == nil {
				return
			}

			eventStream, err = app.operator.Client().Join(context.Background(), &pb.Empty{})
			if err == nil {
				app.log(events.OK, "Connected!")
				break
			} else {
				app.log(events.ERROR, "Could not connect. Reconnecting...")
				time.Sleep(5 * time.Second)
			}
		}

		for {
			event, err := eventStream.Recv()

			if err != nil {
				app.log(events.ERROR, "Disconnected!")
				break
			}

			app.dashboard.RefreshData()

			app.log(events.EventType(event.Type), event.Data)
		}
	}
}

func (app *App) SwitchOperator(oper *operator.Operator) error {
	if app.operator != nil {
		app.operator.Disconnect()
		app.operator = nil
	}

	err := oper.Connect()
	if err != nil {
		return err
	}

	app.operator = oper

	go app.HandleOperatorEvents()

	return nil
}
