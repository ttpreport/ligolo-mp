package main

import (
	"flag"
	"fmt"

	"github.com/ttpreport/ligolo-mp/cmd/client/tui"
	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/config"
	"github.com/ttpreport/ligolo-mp/internal/operator"
	"github.com/ttpreport/ligolo-mp/internal/storage"
)

func main() {
	var credsFile = flag.String("import", "", "Path to credentials file to import")

	flag.Parse()

	cfg := &config.Config{
		Environment: "client",
	}

	storage, err := storage.New(cfg.GetRootAppDir())
	if err != nil {
		panic(fmt.Sprintf("could not connect to storage: %v", err))
	}

	operRepo, _ := operator.NewOperatorRepository(storage)
	certRepo, _ := certificate.NewCertificateRepository(storage)

	certService := certificate.NewCertificateService(certRepo)
	operService := operator.NewOperatorService(cfg, operRepo, certService)

	if *credsFile == "" {
		// cli.Run(operService, certService)

		app := tui.NewApp(operService)
		app.Run()
	} else {
		_, err := operService.NewOperatorFromFile(*credsFile)
		if err != nil {
			panic(err)
		}

		fmt.Println("Credentials successfully imported!")
	}
}
