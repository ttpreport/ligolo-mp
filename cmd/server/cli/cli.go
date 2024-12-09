package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/desertbit/grumble"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/config"
	"github.com/ttpreport/ligolo-mp/internal/operator"
)

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

func Run(cfg *config.Config, certService *certificate.CertificateService, operService *operator.OperatorService) {
	operatorCommand := &grumble.Command{
		Name:      "operator",
		Help:      "Operator management",
		HelpGroup: "Operators",
	}
	App.AddCommand(operatorCommand)

	operatorCommand.AddCommand(&grumble.Command{
		Name:  "list",
		Help:  "List operators",
		Usage: "list",
		Run: func(c *grumble.Context) error {
			operators, err := operService.AllOperators()
			if err != nil {
				return err
			}

			t := table.NewWriter()
			t.SetStyle(table.StyleLight)
			t.SetTitle("Operators")
			t.AppendHeader(table.Row{"Name"})

			for _, operator := range operators {
				t.AppendRow(table.Row{operator.Name})
			}

			c.App.Println(t.Render())

			return nil
		},
	})

	operatorCommand.AddCommand(&grumble.Command{
		Name:  "new",
		Help:  "Add new operator",
		Usage: "new --name [operator_name] --server [ligolo-server:port] --path [path/to/folder/]",
		Flags: func(f *grumble.Flags) {
			f.StringL("name", "", "Operator name")
			f.BoolL("admin", false, "Administrative account")
			f.StringL("server", "", "Server interface to connect to (host:port)")
			f.StringL("path", "", "Folder to save operator config to")

		},
		Run: func(c *grumble.Context) error {
			name := c.Flags.String("name")
			admin := c.Flags.Bool("admin")
			server := c.Flags.String("server")
			path := c.Flags.String("path")

			if name == "" {
				return errors.New("--name is missing")
			}
			if server == "" {
				return errors.New("--server is missing")
			}
			if path == "" {
				return errors.New("--path is missing")
			}

			oper, err := operService.NewOperator(name, admin, server)
			if err != nil {
				return err
			}

			fullPath, err := oper.ToFile(path)
			if err != nil {
				return err
			}

			c.App.Println(fmt.Sprintf("Operator '%s' added. Config is saved to '%s'", name, fullPath))

			return nil
		},
	})

	operatorCommand.AddCommand(&grumble.Command{
		Name:  "delete",
		Help:  "Delete operator",
		Usage: "delete --id [operator_id]",
		Flags: func(f *grumble.Flags) {
			f.StringL("id", "", "Operator ID")
		},
		Run: func(c *grumble.Context) error {
			id := c.Flags.String("id")

			if id == "" {
				return errors.New("--id is missing")
			}

			oper, err := operService.RemoveOperator(id)
			if err != nil {
				return err
			}

			c.App.Println(fmt.Sprintf("Operator '%s' removed.", oper.Name))

			return nil
		},
	})

	certCommand := &grumble.Command{
		Name:      "certificate",
		Help:      "Certificate management",
		HelpGroup: "Certificates",
	}
	App.AddCommand(certCommand)

	certCommand.AddCommand(&grumble.Command{
		Name:  "list",
		Help:  "List certificates",
		Usage: "list",
		Run: func(c *grumble.Context) error {
			certificates, err := certService.GetAll()
			if err != nil {
				return err
			}

			t := table.NewWriter()
			t.SetStyle(table.StyleLight)
			t.SetTitle("Certificates")
			t.AppendHeader(table.Row{"Name", "Certificate", "Key"})

			for _, cert := range certificates {
				t.AppendRow(table.Row{cert.Name, cert.Certificate, cert.Key})
			}

			c.App.Println(t.Render())

			return nil
		},
	})

	certCommand.AddCommand(&grumble.Command{
		Name:  "delete",
		Help:  "Delete certificate",
		Usage: "delete --name [certificate_name]",
		Flags: func(f *grumble.Flags) {
			f.StringL("name", "", "Certificate name")
		},
		Run: func(c *grumble.Context) error {
			name := c.Flags.String("name")

			if name == "" {
				return errors.New("--name is missing")
			}

			if err := certService.Remove(name); err != nil {
				return err
			}

			c.App.Println(fmt.Sprintf("Certificate '%s' removed.", name))

			return nil
		},
	})

	// Grumble doesn't like cli args
	os.Args = []string{}
	App.Run()
}
