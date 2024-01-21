package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/desertbit/grumble"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/ttpreport/ligolo-mp/internal/common/config"
	"github.com/ttpreport/ligolo-mp/internal/common/operator"
	"github.com/ttpreport/ligolo-mp/internal/core/certs"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
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

func Run(cfg *config.Config, storage *storage.Store) {
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
			operators, err := storage.GetOperators()
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
			f.StringL("server", "", "Server interface to connect to (host:port)")
			f.StringL("path", "", "Folder to save operator config to")

		},
		Run: func(c *grumble.Context) error {
			name := c.Flags.String("name")
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

			dbCaData, err := storage.GetCert("_ca_")
			if err != nil {
				return err
			}

			cert, key, err := certs.GenerateCert(name, dbCaData.Certificate, dbCaData.Key)
			if err != nil {
				return err
			}

			certId, err := storage.AddCert(name, cert, key)
			if err != nil {
				return err
			}

			if err = storage.AddOperator(name, certId); err != nil {
				storage.DelCert(name)
				return err
			}

			fullPath, err := operator.SaveToFile(path, name, server, dbCaData.Certificate, cert, key)
			if err != nil {
				storage.DelCert(name)
				storage.DelOperator(name)
				return err
			}

			c.App.Println(fmt.Sprintf("Operator '%s' added. Config is saved to '%s'", name, fullPath))

			return nil
		},
	})

	operatorCommand.AddCommand(&grumble.Command{
		Name:  "delete",
		Help:  "Delete operator",
		Usage: "delete --name [operator_name]",
		Flags: func(f *grumble.Flags) {
			f.StringL("name", "", "Operator name")
		},
		Run: func(c *grumble.Context) error {
			name := c.Flags.String("name")

			if name == "" {
				return errors.New("--name is missing")
			}

			var err error
			if err = storage.DelCert(name); err != nil {
				return err
			}
			if err = storage.DelOperator(name); err != nil {
				return err
			}

			return nil
		},
	})

	// Grumble doesn't like cli args
	os.Args = []string{}
	App.Run()
}
