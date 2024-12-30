package tui

import (
	"context"
	"fmt"
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
	tview.Application
	root         *tview.Flex
	layout       *tview.Flex
	pages        *tview.Pages
	logs         *widgets.LogsWidget
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
		Application: *tview.NewApplication(),
		root:        tview.NewFlex(),
		layout:      tview.NewFlex(),
		pages:       tview.NewPages(),
		logs:        widgets.NewLogsWidget(),
		credentials: pages.NewCredentialsPage(),
		dashboard:   pages.NewDashboardPage(),
		admin:       pages.NewAdminPage(),
		navbar:      widgets.NewNavBar(),
		operService: operService,
	}

	app.root.SetDirection(tview.FlexRow).
		AddItem(app.layout, 0, 99, true).
		AddItem(app.navbar, 0, 1, false)

	app.initCredentials()
	app.initDashboard()
	app.initAdmin()

	app.layout.SetDirection(tview.FlexRow)
	app.layout.AddItem(app.pages, 0, 80, false)
	app.layout.AddItem(app.logs, 0, 20, true)

	app.SwitchToPage(app.credentials)

	go app.autoRedraw()
	go app.autoRefresh()

	return app
}

func (app *App) autoRedraw() {
	tick := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-tick.C:
			app.Draw()
		}
	}
}

func (app *App) autoRefresh() {
	ticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-ticker.C:
			if app.operator.IsConnected() {
				app.dashboard.RefreshData()
				app.admin.RefreshData()
			}
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
		app.admin.SetOperator(oper)
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
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().GetSessions(ctx, &pb.Empty{})
		if err != nil {
			return nil, err
		}

		return r.Sessions, nil
	})

	app.dashboard.SetDisconnectFunc(func() {
		app.operator.Disconnect()
		app.log(events.ERROR, fmt.Sprintf("Disconnected from %s", app.operator.Server))

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
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().StartRelay(ctx, &pb.StartRelayReq{
			SessionID: sess.ID,
		})
		return err
	})

	app.dashboard.SetSessionStopFunc(func(sess *pb.Session) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().StopRelay(ctx, &pb.StopRelayReq{
			SessionID: sess.ID,
		})
		return err
	})

	app.dashboard.SetSessionRenameFunc(func(sess *pb.Session, alias string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().RenameSession(ctx, &pb.RenameSessionReq{
			SessionID: sess.ID,
			Alias:     alias,
		})
		return err
	})

	app.dashboard.SetSessionAddRouteFunc(func(sess *pb.Session, cidr string, loopback bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
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
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().DelRoute(ctx, &pb.DelRouteReq{
			SessionID: sess.ID,
			Cidr:      cidr,
		})
		return err
	})

	app.dashboard.SetSessionAddRedirectorFunc(func(sess *pb.Session, from string, to string, proto string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
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
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().DelRedirector(ctx, &pb.DelRedirectorReq{
			SessionID:    sess.ID,
			RedirectorID: redirectorID,
		})
		return err
	})

	app.dashboard.SetSessionKillFunc(func(sess *pb.Session) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().KillSession(ctx, &pb.KillSessionReq{
			SessionID: sess.ID,
		})
		return err
	})
}

func (app *App) initAdmin() {
	app.admin.SetSwitchbackFunc(func() {
		app.SwitchToPage(app.dashboard)
	})

	app.admin.SetOperatorsFunc(func() ([]*pb.Operator, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().GetOperators(ctx, &pb.Empty{})
		if err != nil {
			return nil, err
		}

		return r.Operators, nil
	})

	app.admin.SetCertificatesFunc(func() ([]*pb.Cert, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().GetCerts(ctx, &pb.Empty{})
		if err != nil {
			return nil, err
		}

		return r.Certs, nil
	})

	app.admin.SetExportOperatorFunc(func(name string, path string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().ExportOperator(ctx, &pb.ExportOperatorReq{
			Name: name,
		})
		if err != nil {
			return "", err
		}

		fullPath := filepath.Join(path, fmt.Sprintf("%s_%s_ligolo-mp.json", r.Operator.Name, r.Operator.Server))
		if err = os.WriteFile(fullPath, r.Config, os.ModePerm); err != nil {
			return "", err
		}

		return fullPath, nil
	})

	app.admin.SetAddOperatorFunc(func(name string, isAdmin bool, server string) (*pb.Operator, *pb.OperatorCredentials, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().AddOperator(ctx, &pb.AddOperatorReq{
			Operator: &pb.Operator{
				Name:    name,
				IsAdmin: isAdmin,
				Server:  server,
			},
		})
		if err != nil {
			return nil, nil, err
		}

		return r.Operator, r.Credentials, nil
	})

	app.admin.SetDelOperatorFunc(func(name string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		_, err := app.operator.Client().DelOperator(ctx, &pb.DelOperatorReq{
			Name: name,
		})

		return err
	})

	app.admin.SetPromoteOperatorFunc(func(name string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		_, err := app.operator.Client().PromoteOperator(ctx, &pb.PromoteOperatorReq{
			Name: name,
		})

		return err
	})

	app.admin.SetDemoteOperatorFunc(func(name string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		_, err := app.operator.Client().DemoteOperator(ctx, &pb.DemoteOperatorReq{
			Name: name,
		})

		return err
	})

	app.admin.SetRegenCertFunc(func(name string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		_, err := app.operator.Client().RegenCert(ctx, &pb.RegenCertReq{
			Name: name,
		})

		return err
	})

	app.admin.SetDisconnectFunc(func() {
		app.operator.Disconnect()
		app.log(events.ERROR, fmt.Sprintf("Disconnected from %s", app.operator.Server))

		app.operator = nil

		app.SwitchToPage(app.credentials)
	})

	app.pages.AddPage(app.credentials.GetID(), app.credentials, true, false)
}

func (app *App) log(severity events.EventType, data string) {
	app.logs.Append(severity, data)
}

func (app *App) HandleOperatorEvents() {
	defer func() {
		app.ShowError("Disconnected from the server", func() {
			app.SwitchToPage(app.credentials)
		})
	}()

	eventStream, err := app.operator.Client().Join(context.Background(), &pb.Empty{})
	if err != nil {
		app.log(events.ERROR, fmt.Sprintf("Could not join event stream: %s", err))
		return
	}

	for {
		event, err := eventStream.Recv()
		if err != nil {
			return
		}

		app.dashboard.RefreshData()
		app.admin.RefreshData()

		app.log(events.EventType(event.Type), event.Data)
	}
}

func (app *App) SwitchOperator(oper *operator.Operator) error {
	if app.operator != nil {
		app.log(events.ERROR, fmt.Sprintf("Disconnected from %s", app.operator.Server))
		app.operator.Disconnect()
		app.operator = nil
	}

	app.log(events.OK, fmt.Sprintf("Connecting to %s as %s", oper.Server, oper.Name))

	err := oper.Connect()
	if err != nil {
		app.log(events.OK, fmt.Sprintf("Could not connect to %s: %s", oper.Server, err))
		return err
	}

	app.log(events.OK, fmt.Sprintf("Connected to %s", oper.Server))

	app.operator = oper
	go app.HandleOperatorEvents()

	return nil
}

func (app *App) ShowError(text string, done func()) {
	modal := modals.NewErrorModal()
	modal.SetText(text)
	modal.SetDoneFunc(func(_ int, _ string) {
		app.pages.RemovePage(modal.GetID())

		if done != nil {
			done()
		}
	})
	app.pages.AddPage(modal.GetID(), modal, true, true)
}

func (app *App) Run() error {
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch key := event.Key(); key {
		case utils.AppInterruptKey.Key:
			return tcell.NewEventKey(key, 0, tcell.ModNone)
		case utils.AppExitKey.Key:
			app.Stop()
			return nil
		}

		return event
	})

	app.credentials.RefreshData()

	if err := app.SetRoot(app.root, true).SetFocus(app.pages).EnablePaste(true).Run(); err != nil {
		return err
	}

	return nil
}
