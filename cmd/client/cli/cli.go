package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/desertbit/grumble"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/manifoldco/promptui"
	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/operator"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

func printErr(format string, args ...any) {
	color.Set(color.FgHiRed)
	App.Println("[-] " + fmt.Sprintf(format+"\n", args...))
	color.Unset()
}

func printWarn(format string, args ...any) {
	color.Set(color.FgHiYellow)
	App.Println("[!] " + fmt.Sprintf(format+"\n", args...))
	color.Unset()
}

func printOk(format string, args ...any) {
	color.Set(color.FgHiGreen)
	App.Println("[+] " + fmt.Sprintf(format+"\n", args...))
	color.Unset()
}

var App = grumble.New(&grumble.Config{
	Name:                  "ligolo-mp",
	Description:           "Ligolo-mp - more specialized version of ligolo-ng",
	HelpHeadlineUnderline: true,
	HelpSubCommands:       true,
	PromptColor:           color.New(color.FgHiBlue),
})

func init() {
	App.SetPrintASCIILogo(func(a *grumble.App) {
		a.Println("      __    _             __                          ")
		a.Println("     / /   (_)___ _____  / /___        ____ ___  ____ ")
		a.Println("    / /   / / __ `/ __ \\/ / __ \\______/ __ `__ \\/ __ \\")
		a.Println("   / /___/ / /_/ / /_/ / / /_/ /_____/ / / / / / /_/ /")
		a.Println("  /_____/_/\\__, /\\____/_/\\____/     /_/ /_/ /_/ .___/ ")
		a.Println("          /____/                             /_/      ")
		a.Println("")
		a.Println("MP-version slapped together by @ttpreport. Original netcode by @Nicocha30.")
		a.Println("")
	})
}

func selectCredentials(operatorService *operator.OperatorService) (*operator.Operator, error) {
	credentials, err := operatorService.AllOperators()
	if err != nil {
		return nil, err
	}

	if len(credentials) < 1 {
		return nil, errors.New("no credentials available")
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "-> {{ .Name | red }}",
		Inactive: "{{ .Name | cyan }}",
		Selected: "Selected: {{ .Name | green }}",
		Details: `
--------- Agents ----------
Name:   {{ .Name }}
Server: {{ .Server }}
`,
	}

	prompt := promptui.Select{
		Label:     "Select credentials",
		Items:     credentials,
		Templates: templates,
		Size:      4,
	}

	index, _, err := prompt.Run()

	if err != nil {
		panic(err)
	}

	cred := credentials[index]

	return cred, nil
}

func getSessions(oper *operator.Operator) ([]*pb.Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	r, err := oper.Client().GetSessions(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	return r.Sessions, nil
}

func selectSession(sessions []*pb.Session) (*pb.Session, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ .ID }}",
		Active:   "-> {{ if not .Alias }} {{ .Hostname }} {{ else }} {{ .Alias }} {{ end }}",
		Inactive: "{{ if not .Alias }} {{ .Hostname }} {{ else }} {{ .Alias }} {{ end }}",
		Selected: "Selected: {{ if not .Alias }} {{ .Hostname }} {{ else }} {{ .Alias }} {{ end }}",
		Details: `
--------- Details ----------
ID: {{ .ID }}
Alias: {{ .Alias }}
Hostname: {{ .Hostname }}

{{range $i, $r := .Interfaces}}
{{range $j, $k := $r.IPs}}IP {{$i}} ({{$j}}): {{ $k }}
{{ end }}
{{ end }}`,
	}

	prompt := promptui.Select{
		Label:     "Select session",
		Items:     sessions,
		Templates: templates,
		Size:      4,
	}

	index, _, err := prompt.Run()

	return sessions[index], err
}

func selectRedirector(sessions []*pb.Session) (*pb.Session, *pb.Redirector, error) {
	var redirectors []struct {
		Session    *pb.Session
		Redirector *pb.Redirector
	}

	for _, session := range sessions {
		for _, redirector := range session.Redirectors {
			redirectors = append(redirectors, struct {
				Session    *pb.Session
				Redirector *pb.Redirector
			}{
				Session:    session,
				Redirector: redirector,
			})
		}
	}

	if len(redirectors) < 1 {
		return nil, nil, errors.New("no redirectors available")
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "-> {{ .Redirector.From }} --{{ .Redirector.Protocol }}-> {{ .Redirector.To }}",
		Inactive: "{{ .Redirector.From }} --{{ .Redirector.Protocol }}-> {{ .Redirector.To }}",
		Selected: "Selected: {{ .Redirector.From }} --{{ .Redirector.Protocol }}-> {{ .Redirector.To }}",
	}

	prompt := promptui.Select{
		Label:     "Select redirector",
		Items:     redirectors,
		Templates: templates,
		Size:      4,
	}

	index, _, err := prompt.Run()

	return redirectors[index].Session, redirectors[index].Redirector, err
}

func Run(operService *operator.OperatorService, certService *certificate.CertificateService) {
	operator, err := selectCredentials(operService)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	err = operator.Connect()
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	defer operator.Disconnect()

	go handleEvents(operator)

	relayCommand := &grumble.Command{
		Name:      "relay",
		Help:      "Relay management",
		HelpGroup: "Relay",
	}
	App.AddCommand(relayCommand)

	relayCommand.AddCommand(&grumble.Command{
		Name:  "start",
		Help:  "Start relaying traffic through session",
		Usage: "relay start",
		Run: func(c *grumble.Context) error {
			var sessions []*pb.Session
			allSessions, err := getSessions(operator)
			if err != nil {
				return err
			}

			for _, session := range allSessions {
				if !session.IsRelaying && session.IsConnected {
					sessions = append(sessions, session)
				}
			}

			if len(sessions) < 1 {
				return errors.New("no sessions available")
			}

			selectedSession, err := selectSession(sessions)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err = operator.Client().StartRelay(ctx, &pb.StartRelayReq{SessionID: selectedSession.ID})

			if err != nil {
				return err
			}

			return nil
		},
	})

	relayCommand.AddCommand(&grumble.Command{
		Name:  "stop",
		Help:  "Stop relaying traffic through session",
		Usage: "relay stop",
		Run: func(c *grumble.Context) error {
			var sessions []*pb.Session
			allSessions, err := getSessions(operator)
			if err != nil {
				return err
			}

			for _, session := range allSessions {
				if session.IsConnected && session.IsRelaying {
					sessions = append(sessions, session)
				}
			}

			if len(sessions) < 1 {
				return errors.New("no sessions available")
			}

			selectedSession, err := selectSession(sessions)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err = operator.Client().StopRelay(ctx, &pb.StopRelayReq{SessionID: selectedSession.ID})

			if err != nil {
				return err
			}

			return nil
		},
	})

	sessionCommand := &grumble.Command{
		Name:      "session",
		Help:      "Session management",
		HelpGroup: "Sessions",
	}
	App.AddCommand(sessionCommand)

	sessionCommand.AddCommand(&grumble.Command{
		Name:  "list",
		Help:  "List sessions",
		Usage: "session list",
		Run: func(c *grumble.Context) error {
			var sessions []*pb.Session
			sessions, err := getSessions(operator)
			if err != nil {
				return err
			}

			if len(sessions) < 1 {
				return errors.New("no sessions available")
			}

			t := table.NewWriter()
			t.SetStyle(table.StyleLight)
			t.SetTitle("Sessions")
			t.AppendHeader(table.Row{"Alias", "Hostname", "Interfaces", "Routes", "Redirectors", "Connected", "Relaying"})

			for _, session := range sessions {
				var routes []string
				for _, route := range session.Tun.Routes {
					ip := route.Cidr
					if route.IsLoopback {
						ip += " (loopback)"
					}
					routes = append(routes, ip)
				}

				var IPs []string
				for _, iface := range session.Interfaces {
					IPs = append(IPs, iface.IPs...)
				}

				var redirectors []string
				for _, redirector := range session.Redirectors {
					redirectors = append(redirectors, fmt.Sprintf("%s --(%s)-> %s", redirector.From, redirector.Protocol, redirector.To))
				}

				t.AppendRow(table.Row{
					session.Alias,
					session.Hostname,
					strings.Join(IPs, "\n"),
					strings.Join(routes, "\n"),
					strings.Join(redirectors, "\n"),
					session.IsConnected,
					session.IsRelaying,
				})

			}

			c.App.Println(t.Render())

			return nil
		},
	})

	sessionCommand.AddCommand(&grumble.Command{
		Name:  "rename",
		Help:  "Rename session (change alias)",
		Usage: "session rename new-alias",
		Args: func(a *grumble.Args) {
			a.String("alias", "session alias")
		},
		Run: func(c *grumble.Context) error {
			sessions, err := getSessions(operator)
			if err != nil {
				return err
			}

			if len(sessions) < 1 {
				return errors.New("no sessions available")
			}

			selectedSession, err := selectSession(sessions)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			_, err = operator.Client().RenameSession(ctx, &pb.RenameSessionReq{
				SessionID: selectedSession.ID,
				Alias:     c.Args.String("alias"),
			})

			if err != nil {
				return err
			}

			return nil
		},
	})

	routeCommand := &grumble.Command{
		Name:      "route",
		Help:      "Routes management",
		HelpGroup: "Routes",
	}
	App.AddCommand(routeCommand)

	routeCommand.AddCommand(&grumble.Command{
		Name:  "new",
		Help:  "Add route to the session",
		Usage: "route new route_1 [route_2 route_3 ... route_N]",
		Args: func(a *grumble.Args) {
			a.StringList("routes", "Routes list")
		},
		Flags: func(f *grumble.Flags) {
			f.BoolL("loopback", false, "set route to remote's loopback")
		},
		Run: func(c *grumble.Context) error {
			sessions, err := getSessions(operator)
			if err != nil {
				return err
			}

			if len(sessions) < 1 {
				return errors.New("no sessions available")
			}

			selectedSession, err := selectSession(sessions)
			if err != nil {
				return err
			}

			isLoopback := c.Flags.Bool("loopback")

			for _, route := range c.Args.StringList("routes") {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				r, err := operator.Client().AddRoute(ctx, &pb.AddRouteReq{
					SessionID: selectedSession.ID,
					Route:     &pb.Route{Cidr: route, IsLoopback: isLoopback},
				})
				if err != nil {
					return err
				}

				if r.OverlappingSession != nil {
					return fmt.Errorf("your route overlaps with routing of TUN %s", r.OverlappingSession.ID)
				}
			}

			return nil
		},
	})

	routeCommand.AddCommand(&grumble.Command{
		Name:  "delete",
		Help:  "Delete route from session",
		Usage: "route delete route_1 [route_2 route_3 ... route_N]",
		Args: func(a *grumble.Args) {
			a.StringList("routes", "Routes list")
		},
		Run: func(c *grumble.Context) error {
			sessions, err := getSessions(operator)
			if err != nil {
				return err
			}

			if len(sessions) < 1 {
				return errors.New("no sessions available")
			}

			selectedSession, err := selectSession(sessions)
			if err != nil {
				return err
			}

			for _, route := range c.Args.StringList("routes") {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				_, err := operator.Client().DelRoute(ctx, &pb.DelRouteReq{
					SessionID: selectedSession.ID,
					Cidr:      route,
				})
				if err != nil {
					return err
				}
			}

			return nil
		},
	})

	redirectorCommand := &grumble.Command{
		Name:      "redirector",
		Help:      "Redirector management",
		HelpGroup: "Redirector",
	}
	App.AddCommand(redirectorCommand)

	redirectorCommand.AddCommand(&grumble.Command{
		Name:  "new",
		Help:  "Set up traffic redirector from session to destination",
		Usage: "redirector add --from [listening_address:port] --to [destination_address:port] --tcp/--udp",
		Flags: func(f *grumble.Flags) {
			f.BoolL("tcp", false, "Use TCP listener")
			f.BoolL("udp", false, "Use UDP listener")
			f.StringL("from", "", "The session listening address:port")
			f.StringL("to", "", "Where to redirect connections")

		},
		Run: func(c *grumble.Context) error {
			var netProto string
			if c.Flags.Bool("tcp") && c.Flags.Bool("udp") {
				return errors.New("choose TCP or UDP, not both")
			}
			if c.Flags.Bool("tcp") {
				netProto = "tcp"
			}
			if c.Flags.Bool("udp") {
				netProto = "udp"
			}
			if netProto == "" {
				netProto = "tcp" // Use TCP by default.
			}

			sessions, err := getSessions(operator)
			if err != nil {
				return err
			}

			if len(sessions) < 1 {
				return errors.New("no sessions available")
			}

			selectedSession, err := selectSession(sessions)
			if err != nil {
				return err
			}

			from := c.Flags.String("from")
			to := c.Flags.String("to")

			// Check if specified IP is valid.
			if _, _, err := net.SplitHostPort(from); err != nil {
				return fmt.Errorf("--from is invalid: %s", err)
			}
			if _, _, err := net.SplitHostPort(to); err != nil {
				return fmt.Errorf("--to is invalid: %s", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err = operator.Client().AddRedirector(ctx, &pb.AddRedirectorReq{
				SessionID: selectedSession.ID,
				Protocol:  netProto,
				From:      from,
				To:        to,
			})

			if err != nil {
				return err
			}

			return nil
		},
	})

	redirectorCommand.AddCommand(&grumble.Command{
		Name:  "delete",
		Help:  "Delete a redirector",
		Usage: "listener delete",
		Run: func(c *grumble.Context) error {
			sessions, err := getSessions(operator)
			if err != nil {
				return err
			}

			selectedSession, selectedRedirector, err := selectRedirector(sessions)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err = operator.Client().DelRedirector(ctx, &pb.DelRedirectorReq{
				SessionID:    selectedSession.ID,
				RedirectorID: selectedRedirector.ID,
			})

			if err != nil {
				return err
			}

			return nil
		},
	})

	agentCommand := &grumble.Command{
		Name:      "agent",
		Help:      "Agent management",
		HelpGroup: "Agents",
	}
	App.AddCommand(agentCommand)

	agentCommand.AddCommand(&grumble.Command{
		Name:  "generate",
		Help:  "Generate agent binary for current server",
		Usage: "agent generate --save /tmp/agent.exe --server 127.0.0.1:11601 [--os windows --arch amd64 --socks-server 127.0.0.1:1234 --socks-user user --socks-pass password --obfuscate]",
		Flags: func(f *grumble.Flags) {
			f.StringL("save", "", "Path to save the binary")
			f.StringL("server", "", "Agent server")
			f.StringL("os", "windows", "Binary target OS")
			f.StringL("arch", "amd64", "Binary target architecture")
			f.StringL("socks-server", "", "Socks server address for agent")
			f.StringL("socks-user", "", "Socks username for agent")
			f.StringL("socks-pass", "", "Socks password for agent")
			f.BoolL("obfuscate", false, "Enable binary obfuscation")
		},
		Run: func(c *grumble.Context) error {
			save := c.Flags.String("save")
			if save == "" {
				return errors.New("--save is required")
			}

			server := c.Flags.String("server")
			if server == "" {
				return errors.New("--server is required")
			}

			if _, _, err := net.SplitHostPort(server); err != nil {
				return fmt.Errorf("--server is invalid: %s", err)
			}

			goos := c.Flags.String("os")
			goarch := c.Flags.String("arch")

			socksServer := c.Flags.String("socks-server")
			if c.Flags.String("socks-server") != "" {
				if _, _, err := net.SplitHostPort(socksServer); err != nil {
					return fmt.Errorf("--socks-server is invalid: %s", err)
				}
			}

			socksUser := c.Flags.String("socks-user")
			socksPass := c.Flags.String("socks-pass")
			obfuscate := c.Flags.Bool("obfuscate")

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			r, err := operator.Client().GenerateAgent(ctx, &pb.GenerateAgentReq{
				Server:      server,
				GOOS:        goos,
				GOARCH:      goarch,
				SocksServer: socksServer,
				SocksUser:   socksUser,
				SocksPass:   socksPass,
				Obfuscate:   obfuscate,
			})
			if err != nil {
				return err
			}

			if err = os.WriteFile(save, r.AgentBinary, 0755); err != nil {
				return err
			}

			fullPath, err := filepath.Abs(save)
			if err != nil {
				return err
			}

			printOk("Agent binary saved to %s", fullPath)

			return nil
		},
	})

	os.Args = []string{}
	App.Run()
}
