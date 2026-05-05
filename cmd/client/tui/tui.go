package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/modals"
	"github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/pages"
	"github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/utils"
	"github.com/ttpreport/ligolo-mp/v2/cmd/client/tui/widgets"
	"github.com/ttpreport/ligolo-mp/v2/internal/certificate"
	"github.com/ttpreport/ligolo-mp/v2/internal/config"
	"github.com/ttpreport/ligolo-mp/v2/internal/events"
	"github.com/ttpreport/ligolo-mp/v2/internal/operator"
	"github.com/ttpreport/ligolo-mp/v2/internal/session"
	pb "github.com/ttpreport/ligolo-mp/v2/protobuf"
)

type App struct {
	tview.Application
	root         *tview.Flex
	layout       *tview.Flex
	pages        *tview.Pages
	Logs         *widgets.LogsWidget
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
		Logs:        widgets.NewLogsWidget(),
		credentials: pages.NewCredentialsPage(),
		dashboard:   pages.NewDashboardPage(),
		admin:       pages.NewAdminPage(),
		navbar:      widgets.NewNavBar(),
		operService: operService,
	}

	// Enable mouse support
	app.EnableMouse(true)

	// Set up navbar click handler to simulate keyboard events
	app.navbar.SetClickHandler(func(key tcell.Key) {
		// Ensure proper focus for the current page before simulating key event
		switch app.currentPage {
		case app.credentials.GetID():
			// Focus the credentials table so InputHandler processes the event properly
			app.SetFocus(app.credentials.GetTable())
		case app.dashboard.GetID():
			app.SetFocus(app.dashboard)
		case app.admin.GetID():
			app.SetFocus(app.admin)
		}

		// Create and queue the keyboard event - let existing handlers process it
		event := tcell.NewEventKey(key, 0, tcell.ModNone)
		app.QueueEvent(event)
	})

	app.root.SetDirection(tview.FlexRow).
		AddItem(app.layout, 0, 99, true).
		AddItem(app.navbar, 0, 1, false)

	app.initCredentials()
	app.initDashboard()
	app.initAdmin()

	app.layout.SetDirection(tview.FlexRow)
	app.layout.AddItem(app.pages, 0, 80, false)
	app.layout.AddItem(app.Logs, 0, 20, true)

	app.SwitchToPage(app.credentials)

	return app
}

func (app *App) Reset() {
	app.dashboard.Reset()
	app.admin.Reset()

	app.SwitchToPage(app.credentials)
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

	app.credentials.SetConnectFunc(func(oper *operator.Operator) error {
		if oper == nil {
			return nil
		}

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
	app.dashboard.SetApp(&app.Application)

	app.dashboard.SetAdminFunc(func() {
		app.SwitchToPage(app.admin)
	})

	app.dashboard.SetDataFunc(func() ([]*session.Session, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().GetSessions(ctx, &pb.Empty{})
		if err != nil {
			return nil, err
		}

		var sessions []*session.Session
		for _, sess := range r.Sessions {
			sessions = append(sessions, session.ProtoToSession(sess))
		}

		return sessions, nil
	})

	app.dashboard.SetGenerateFunc(func(path string, servers string, goos string, goarch string, obfuscate bool, proxy string, ignoreEnvProxy bool) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()

		r, err := app.operator.Client().GenerateAgent(ctx, &pb.GenerateAgentReq{
			Servers:        servers,
			GOOS:           goos,
			GOARCH:         goarch,
			Obfuscate:      obfuscate,
			ProxyServer:    proxy,
			IgnoreEnvProxy: ignoreEnvProxy,
		})
		if err != nil {
			return "", err
		}

		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			path = filepath.Join(path, "agent.bin")
		}

		if err = os.WriteFile(path, r.AgentBinary, 0755); err != nil {
			return "", err
		}

		return filepath.Abs(path)
	})

	app.dashboard.SetSessionStartFunc(func(sess *session.Session) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().StartRelay(ctx, &pb.StartRelayReq{
			SessionID: sess.ID,
		})
		return err
	})

	app.dashboard.SetSessionStopFunc(func(sess *session.Session) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().StopRelay(ctx, &pb.StopRelayReq{
			SessionID: sess.ID,
		})
		return err
	})

	app.dashboard.SetSessionRenameFunc(func(sess *session.Session, alias string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().RenameSession(ctx, &pb.RenameSessionReq{
			SessionID: sess.ID,
			Alias:     alias,
		})
		return err
	})

	app.dashboard.SetSessionAddRouteFunc(func(sess *session.Session, cidr string, metric int, loopback bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		_, err := app.operator.Client().AddRoute(ctx, &pb.AddRouteReq{
			SessionID: sess.ID,
			Route: &pb.Route{
				Cidr:       cidr,
				Metric:     int32(metric),
				IsLoopback: loopback,
			},
		})
		return err
	})

	app.dashboard.SetSessionEditRouteFunc(func(sess *session.Session, routeID string, cidr string, metric int, loopback bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().EditRoute(ctx, &pb.EditRouteReq{
			SessionID: sess.ID,
			RouteID:   routeID,
			Route: &pb.Route{
				Cidr:       cidr,
				Metric:     int32(metric),
				IsLoopback: loopback,
			},
		})
		return err
	})

	app.dashboard.SetSessionMoveRouteFunc(func(sess *session.Session, routeID string, targetSessionID string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().MoveRoute(ctx, &pb.MoveRouteReq{
			OldSessionID: sess.ID,
			RouteID:      routeID,
			NewSessionID: targetSessionID,
		})
		return err
	})

	app.dashboard.SetSessionRemoveRouteFunc(func(sess *session.Session, routeID string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().DelRoute(ctx, &pb.DelRouteReq{
			SessionID: sess.ID,
			RouteID:   routeID,
		})
		return err
	})

	app.dashboard.SetSessionAddRedirectorFunc(func(sess *session.Session, from string, to string, proto string) error {
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

	app.dashboard.SetSessionRemoveRedirectorFunc(func(sess *session.Session, redirectorID string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().DelRedirector(ctx, &pb.DelRedirectorReq{
			SessionID:    sess.ID,
			RedirectorID: redirectorID,
		})
		return err
	})

	app.dashboard.SetSessionKillFunc(func(sess *session.Session) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_, err := app.operator.Client().KillSession(ctx, &pb.KillSessionReq{
			SessionID: sess.ID,
		})
		return err
	})

	app.dashboard.SetMetadataFunc(func() (*config.Config, *operator.Operator, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().GetMetadata(ctx, &pb.Empty{})
		if err != nil {
			return nil, nil, err
		}

		config := config.ProtoToConfig(r.Config)
		operator := operator.ProtoToOperator(r.Operator)

		return config, operator, nil
	})

	app.dashboard.SetTracerouteFunc(func(address string) ([]string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().Traceroute(ctx, &pb.TracerouteReq{
			IP: address,
		})
		if err != nil {
			return nil, err
		}

		var trace []string
		var route string
		for _, traceline := range r.Trace {
			if traceline.IsInternal {
				route = fmt.Sprintf("%s routed via session '%s'", address, traceline.Session)
			} else {
				route = fmt.Sprintf("%s routed externally via %s", address, traceline.Iface)

				if traceline.Via != "" {
					route += fmt.Sprintf(" (%s)", traceline.Via)
				}
			}

			trace = append(trace, route)
		}

		return trace, nil
	})
}

func (app *App) initAdmin() {
	app.admin.SetApp(&app.Application)

	app.admin.SetSwitchbackFunc(func() {
		app.SwitchToPage(app.dashboard)
	})

	app.admin.SetOperatorsFunc(func() ([]*operator.Operator, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().GetOperators(ctx, &pb.Empty{})
		if err != nil {
			return nil, err
		}

		var opers []*operator.Operator
		for _, oper := range r.Operators {
			opers = append(opers, operator.ProtoToOperator(oper))
		}

		return opers, nil
	})

	app.admin.SetCertificatesFunc(func() ([]*certificate.Certificate, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().GetCerts(ctx, &pb.Empty{})
		if err != nil {
			return nil, err
		}

		var certs []*certificate.Certificate
		for _, cert := range r.Certs {
			certs = append(certs, certificate.ProtoToCertificate(cert))
		}

		return certs, nil
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

		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			path = filepath.Join(path, fmt.Sprintf("%s_%s_ligolo-mp.json", r.Operator.Name, r.Operator.Server))
		}

		if err = os.WriteFile(path, r.Config, os.ModePerm); err != nil {
			return "", err
		}

		return filepath.Abs(path)
	})

	app.admin.SetAddOperatorFunc(func(name string, isAdmin bool, server string) (*operator.Operator, error) {
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
			return nil, err
		}

		return operator.ProtoToOperator(r.Operator), nil
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

	app.admin.SetMetadataFunc(func() (*config.Config, *operator.Operator, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		r, err := app.operator.Client().GetMetadata(ctx, &pb.Empty{})
		if err != nil {
			return nil, nil, err
		}

		config := config.ProtoToConfig(r.Config)
		operator := operator.ProtoToOperator(r.Operator)

		return config, operator, nil
	})

	app.pages.AddPage(app.credentials.GetID(), app.credentials, true, false)
}

func (app *App) HandleOperatorEvents() {
	defer func() {
		app.Disconnect()
		app.QueueUpdateDraw(func() {
			app.Reset()
			app.ShowError("Disconnected from the server", nil)
		})
	}()

	eventStream, err := app.operator.Client().Join(context.Background(), &pb.Empty{})
	if err != nil {
		slog.Error(fmt.Sprintf("Could not join event stream: %s", err))
		return
	}

	for {
		event, err := eventStream.Recv()
		if err != nil {
			return
		}

		app.dashboard.RefreshData()
		app.admin.RefreshData()

		slog.Log(context.Background(), events.EventType(event.Type).Slog(), event.Data)
	}
}

func (app *App) IsConnected() bool {
	return app.operator != nil && app.operator.IsConnected()
}

func (app *App) Disconnect() {
	if app.IsConnected() {
		app.operator.Disconnect()
		slog.Error(fmt.Sprintf("Disconnected from %s", app.operator.Server))
	}

	app.operator = nil
}

func (app *App) SwitchOperator(oper *operator.Operator) error {
	if app.IsConnected() {
		app.Disconnect()
	}

	slog.Info(fmt.Sprintf("Connecting to %s as %s", oper.Server, oper.Name))

	err := oper.Connect()
	if err != nil {
		slog.Error(fmt.Sprintf("Could not connect to %s: %s", oper.Server, err))
		return err
	}

	slog.Info(fmt.Sprintf("Connected to %s", oper.Server))

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
