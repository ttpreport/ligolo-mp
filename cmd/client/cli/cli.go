package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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
	"github.com/ttpreport/ligolo-mp/internal/common/operator"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

func selectCredentials(store *storage.Store) *operator.Credentials {
	credentials, err := store.GetCreds()
	if err != nil {
		panic(err)
	}

	if len(credentials) < 1 {
		panic("no credentials available")
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

	return &operator.Credentials{
		Name:         cred.Name,
		Server:       cred.Server,
		CACert:       cred.CaCert,
		OperatorCert: cred.OperatorCert,
		OperatorKey:  cred.OperatorKey,
	}
}

func selectAgent(client pb.LigoloClient) (*pb.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	r, err := client.ListAgents(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	if len(r.Agents) < 1 {
		return nil, errors.New("no active agents")
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "-> {{ .Alias | red }}",
		Inactive: "{{ .Alias | cyan }}",
		Selected: "Selected: {{ .Alias | green }}",
		Details: `
--------- Agents ----------
Alias:    {{ .Alias }}
Hostname: {{ .Hostname }}

{{range $i, $r := .IPs}}
IP {{$i}}:	{{ $r }}{{ end }}`,
	}

	prompt := promptui.Select{
		Label:     "Select agent",
		Items:     r.Agents,
		Templates: templates,
		Size:      4,
	}

	index, _, err := prompt.Run()

	return r.Agents[index], err
}

func selectTun(client pb.LigoloClient) (*pb.Tun, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	r, err := client.ListTuns(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	if len(r.Tuns) < 1 {
		return nil, errors.New("no active TUNs")
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "-> {{ .Alias | red }}",
		Inactive: "{{ .Alias | cyan }}",
		Selected: "Selected: {{ .Alias | green }}",
		Details: `
--------- Listener ----------
Alias:	{{ .Alias }}

{{range $i, $r := .Routes}}
Route {{$i}}:	{{ .Cidr }}{{ end }}`,
	}

	prompt := promptui.Select{
		Label:     "Select TUN",
		Items:     r.Tuns,
		Templates: templates,
		Size:      4,
	}

	index, _, err := prompt.Run()

	return r.Tuns[index], err
}

func selectListener(client pb.LigoloClient) (*pb.Listener, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	r, err := client.ListListeners(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}

	if len(r.Listeners) < 1 {
		return nil, errors.New("no active listeners")
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "-> {{ .Alias | red }}",
		Inactive: "{{ .Alias | cyan }}",
		Selected: "Selected: {{ .Alias | green }}",
		Details: `
--------- Listener ----------
{{ "Alias:" | faint }}	{{ .Alias }}
{{ "From:" | faint }}	{{ .From }}
{{ "To:" | faint }}	{{ .To }}`,
	}

	prompt := promptui.Select{
		Label:     "Select listener",
		Items:     r.Listeners,
		Templates: templates,
		Size:      4,
	}

	index, _, err := prompt.Run()

	return r.Listeners[index], err
}

func Run(store *storage.Store) {
	operatorCfg := selectCredentials(store)
	operatorCert, err := tls.X509KeyPair(operatorCfg.OperatorCert, operatorCfg.OperatorKey)
	if err != nil {
		panic(err)
	}

	ca := x509.NewCertPool()
	if ok := ca.AppendCertsFromPEM(operatorCfg.CACert); !ok {
		panic("failed to parse CACert")
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{operatorCert},
		RootCAs:            ca,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}

			options := x509.VerifyOptions{
				Roots: ca,
			}
			if options.Roots == nil {
				return errors.New("no root certificate")
			}
			if _, err := cert.Verify(options); err != nil {
				return err
			}

			return nil
		},
	}

	conn, err := grpc.Dial(operatorCfg.Server,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(20*1024*1024)),
	)
	if err != nil {
		printErr("did not connect: %s", err)
	}
	defer conn.Close()

	client := pb.NewLigoloClient(conn)

	eventStream, err := client.Join(context.Background(), &pb.Empty{})
	if err != nil {
		panic(err)
	}

	go handleEvents(eventStream)

	relayCommand := &grumble.Command{
		Name:      "relay",
		Help:      "Relay management",
		HelpGroup: "Relay",
	}
	App.AddCommand(relayCommand)

	relayCommand.AddCommand(&grumble.Command{
		Name:  "start",
		Help:  "Start relaying connection between agent and tun",
		Usage: "relay start",
		Run: func(c *grumble.Context) error {
			selected_agent, err := selectAgent(client)
			if err != nil {
				return err
			}

			selected_tun, err := selectTun(client)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err = client.RelayStart(ctx, &pb.RelayStartReq{AgentAlias: selected_agent.Alias, TunAlias: selected_tun.Alias})

			if err != nil {
				return err
			}

			return nil
		},
	})

	relayCommand.AddCommand(&grumble.Command{
		Name:  "stop",
		Help:  "Stop relaying connection between agent and tun",
		Usage: "relay stop",
		Run: func(c *grumble.Context) error {
			selected_agent, err := selectAgent(client)

			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err = client.RelayStop(ctx, &pb.RelayStopReq{AgentAlias: selected_agent.Alias})

			if err != nil {
				return err
			}

			return nil
		},
	})

	agentCommand := &grumble.Command{
		Name:      "agent",
		Help:      "Agent management",
		HelpGroup: "Agent",
	}
	App.AddCommand(agentCommand)

	agentCommand.AddCommand(&grumble.Command{
		Name:  "list",
		Help:  "List currently running agents",
		Usage: "agent list",
		Run: func(c *grumble.Context) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			r, err := client.ListAgents(ctx, &pb.Empty{})
			if err != nil {
				return err
			}

			t := table.NewWriter()
			t.SetStyle(table.StyleLight)
			t.SetTitle("Active agents")
			t.AppendHeader(table.Row{"Alias", "Hostname", "Agent interfaces", "Connected TUN", "Routes", "Is loopback"})

			for _, agent := range r.Agents {
				if agent.Tun != nil {
					var routes []string
					for _, route := range agent.Tun.Routes {
						routes = append(routes, route.Cidr)
					}

					t.AppendRow(table.Row{
						agent.Alias,
						agent.Hostname,
						strings.Join(agent.IPs, "\n"),
						agent.Tun.Alias,
						strings.Join(routes, "\n"),
						agent.Tun.IsLoopback,
					})
				} else {
					t.AppendRow(table.Row{
						agent.Alias,
						agent.Hostname,
						strings.Join(agent.IPs, "\n"),
					})
				}
			}

			c.App.Println(t.Render())

			return nil
		},
	})

	agentCommand.AddCommand(&grumble.Command{
		Name:  "rename",
		Help:  "Rename existing agent session",
		Usage: "agent rename new-name",
		Args: func(a *grumble.Args) {
			a.String("name", "New name of the agent")
		},
		Run: func(c *grumble.Context) error {
			agent, err := selectAgent(client)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			_, err = client.RenameAgent(ctx, &pb.RenameAgentReq{
				OldAlias: agent.Alias,
				NewAlias: c.Args.String("name"),
			})

			if err != nil {
				return err
			}

			return nil
		},
	})

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
			r, err := client.GenerateAgent(ctx, &pb.GenerateAgentReq{
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

	tunCommand := &grumble.Command{
		Name:      "tun",
		Help:      "TUN management",
		HelpGroup: "TUNs",
	}
	App.AddCommand(tunCommand)

	tunCommand.AddCommand(&grumble.Command{
		Name:  "list",
		Help:  "List currently running TUNs",
		Usage: "tun list",
		Run: func(c *grumble.Context) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			r, err := client.ListTuns(ctx, &pb.Empty{})
			if err != nil {
				return err
			}

			t := table.NewWriter()
			t.SetStyle(table.StyleLight)
			t.SetTitle("Active TUNs")
			t.AppendHeader(table.Row{"Alias", "Routes", "Is loopback"})

			for _, tun := range r.GetTuns() {
				var routes []string
				for _, route := range tun.Routes {
					routes = append(routes, route.Cidr)
				}
				t.AppendRow(table.Row{tun.Alias, strings.Join(routes, "\n"), tun.IsLoopback})
			}
			c.App.Println(t.Render())
			return nil
		},
	})

	tunCommand.AddCommand(&grumble.Command{
		Name:  "new",
		Help:  "Create new TUN",
		Usage: "tun new",
		Flags: func(f *grumble.Flags) {
			f.StringL("name", "", "System name of the TUN")
			f.BoolL("loopback", false, "Make tun route to agent's localhost")
		},
		Run: func(c *grumble.Context) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err := client.NewTun(ctx, &pb.NewTunReq{
				Name:       c.Flags.String("name"),
				IsLoopback: c.Flags.Bool("loopback"),
			})
			if err != nil {
				return err
			}

			return nil
		},
	})

	tunCommand.AddCommand(&grumble.Command{
		Name:  "rename",
		Help:  "Rename created TUN",
		Usage: "tun rename new-name",
		Args: func(a *grumble.Args) {
			a.String("name", "New name for TUN")
		},
		Run: func(c *grumble.Context) error {
			tun, err := selectTun(client)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			_, err = client.RenameTun(ctx, &pb.RenameTunReq{
				OldAlias: tun.Alias,
				NewAlias: c.Args.String("name"),
			})

			if err != nil {
				return err
			}

			return nil
		},
	})

	tunCommand.AddCommand(&grumble.Command{
		Name:  "delete",
		Help:  "Delete the TUN",
		Usage: "tun delete",
		Run: func(c *grumble.Context) error {
			tun, err := selectTun(client)

			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err = client.DelTun(ctx, &pb.DelTunReq{
				TunAlias: tun.Alias,
			})

			if err != nil {
				return err
			}

			return nil
		},
	})

	tunRouteCommand := &grumble.Command{
		Name:      "route",
		Help:      "Route management for a TUN",
		HelpGroup: "TUNs",
	}
	tunCommand.AddCommand(tunRouteCommand)

	tunRouteCommand.AddCommand(&grumble.Command{
		Name:  "new",
		Help:  "Add route to the TUN",
		Usage: "tun route new route_1 [route_2 route_3 ... route_N]",
		Args: func(a *grumble.Args) {
			a.StringList("routes", "Routes list")
		},
		Run: func(c *grumble.Context) error {
			var err error
			tun, err := selectTun(client)

			if err != nil {
				return err
			}

			for _, route := range c.Args.StringList("routes") {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				r, err := client.NewRoute(ctx, &pb.NewRouteReq{
					TunAlias: tun.Alias,
					Cidr:     route,
				})
				if err != nil {
					return err
				}

				if r.OverlappingTun != nil {
					return fmt.Errorf("your route overlaps with routing of TUN %s", r.OverlappingTun.Alias)
				}
			}

			return nil
		},
	})

	tunRouteCommand.AddCommand(&grumble.Command{
		Name:  "delete",
		Help:  "Delete route from the TUN",
		Usage: "tun route delete route_1 [route_2 route_3 ... route_N]",
		Args: func(a *grumble.Args) {
			a.StringList("routes", "Routes list")
		},
		Run: func(c *grumble.Context) error {
			var err error
			tun, err := selectTun(client)

			if err != nil {
				return err
			}

			for _, route := range c.Args.StringList("routes") {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel()
				_, err = client.DelRoute(ctx, &pb.DelRouteReq{
					TunAlias: tun.Alias,
					Cidr:     route,
				})

				if err != nil {
					printErr("Error deleting route %s: %s", route, err)
				}
			}

			return nil
		},
	})

	listenerCommand := &grumble.Command{
		Name:      "listener",
		Help:      "Listener management",
		HelpGroup: "Listener",
	}
	App.AddCommand(listenerCommand)

	listenerCommand.AddCommand(&grumble.Command{
		Name:  "list",
		Help:  "List currently running listeners",
		Usage: "listener list",
		Run: func(c *grumble.Context) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			r, err := client.ListListeners(ctx, &pb.Empty{})
			if err != nil {
				printErr("RPC failed: %s", err)
			}

			t := table.NewWriter()
			t.SetStyle(table.StyleLight)
			t.SetTitle("Active listeners")
			t.AppendHeader(table.Row{"Alias", "Agent", "From", "To"})

			for _, listener := range r.GetListeners() {
				t.AppendRow(table.Row{listener.Alias, listener.Agent.Alias, listener.From, listener.To})
			}

			c.App.Println(t.Render())
			return nil
		},
	})

	listenerCommand.AddCommand(&grumble.Command{
		Name:  "new",
		Help:  "Listen on the agent and redirect connections to the desired address",
		Usage: "listener add --from [listening_address:port] --to [destination_address:port] --tcp/--udp",
		Flags: func(f *grumble.Flags) {
			f.BoolL("tcp", false, "Use TCP listener")
			f.BoolL("udp", false, "Use UDP listener")
			f.StringL("from", "", "The agent listening address:port")
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

			agent, err := selectAgent(client)
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
			_, err = client.NewListener(ctx, &pb.NewListenerReq{
				AgentAlias: agent.Alias,
				Protocol:   netProto,
				From:       from,
				To:         to,
			})

			if err != nil {
				return err
			}

			return nil
		},
	})

	listenerCommand.AddCommand(&grumble.Command{
		Name:  "delete",
		Help:  "Delete a listener",
		Usage: "listener delete",
		Run: func(c *grumble.Context) error {
			listener, err := selectListener(client)

			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err = client.DelListener(ctx, &pb.DelListenerReq{
				AgentAlias:    listener.Agent.Alias,
				ListenerAlias: listener.Alias,
			})

			if err != nil {
				return err
			}

			return nil
		},
	})

	certCommand := &grumble.Command{
		Name:      "certificate",
		Help:      "Certificate management",
		HelpGroup: "Certificate",
	}
	App.AddCommand(certCommand)

	certCommand.AddCommand(&grumble.Command{
		Name:  "list",
		Help:  "List certificates",
		Usage: "certificate list",
		Run: func(c *grumble.Context) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			r, err := client.ListCerts(ctx, &pb.Empty{})
			if err != nil {
				return err
			}

			t := table.NewWriter()
			t.SetStyle(table.StyleLight)
			t.SetTitle("Certificates")
			t.AppendHeader(table.Row{"Name", "Expiration date"})

			for _, cert := range r.Certs {
				t.AppendRow(table.Row{cert.Name, cert.ExpiryDate})
			}
			c.App.Println(t.Render())

			return nil
		},
	})

	certCommand.AddCommand(&grumble.Command{
		Name:  "regenerate",
		Help:  "Regenerate certificate",
		Usage: "certificate regenerate certificate_name",
		Args: func(a *grumble.Args) {
			a.String("name", "Certificate name")
		},
		Run: func(c *grumble.Context) error {
			name := c.Args.String("name")

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			_, err := client.RegenCerts(ctx, &pb.RegenCertsReq{Name: name})
			if err != nil {
				return err
			}

			printOk("Certificate %s regenerated", name)

			return nil
		},
	})

	// Grumble doesn't like cli args
	os.Args = []string{}
	App.Run()
}
