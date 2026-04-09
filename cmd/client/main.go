package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ttpreport/ligolo-mp/v2/cmd/client/tui"
	"github.com/ttpreport/ligolo-mp/v2/internal/certificate"
	"github.com/ttpreport/ligolo-mp/v2/internal/config"
	"github.com/ttpreport/ligolo-mp/v2/internal/crl"
	"github.com/ttpreport/ligolo-mp/v2/internal/operator"
	"github.com/ttpreport/ligolo-mp/v2/internal/storage"
	"github.com/ttpreport/ligolo-mp/v2/internal/version"
	"github.com/ttpreport/ligolo-mp/v2/pkg/logger"
)

func main() {
	var verbose = flag.Bool("v", false, "enable verbose mode")
	var showVersion = flag.Bool("version", false, "print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ligolo-mp-client %s\n\nUsage:\n", version.Version)
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("ligolo-mp-client %s\n", version.Version)
		os.Exit(0)
	}

	loggingOpts := &slog.HandlerOptions{}
	if *verbose {
		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelDebug)
		loggingOpts = &slog.HandlerOptions{
			Level: lvl,
		}
	} else {
		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelInfo)
		loggingOpts = &slog.HandlerOptions{
			Level: lvl,
		}
	}
	logHandler := slog.New(slog.NewTextHandler(os.Stdout, loggingOpts))
	slog.SetDefault(logHandler)

	cfg := &config.Config{
		Environment: "client",
	}

	storage, err := storage.New(cfg.GetRootAppDir())
	if err != nil {
		panic(fmt.Sprintf("could not connect to storage: %v", err))
	}

	operRepo, err := operator.NewOperatorRepository(storage)
	if err != nil {
		panic(err)
	}

	certRepo, err := certificate.NewCertificateRepository(storage)
	if err != nil {
		panic(err)
	}

	crlRepo, err := crl.NewCRLRepository(storage)
	if err != nil {
		panic(err)
	}

	crlService := crl.NewCRLService(crlRepo)
	certService := certificate.NewCertificateService(certRepo, crlService)
	operService := operator.NewOperatorService(cfg, operRepo, certService)

	app := tui.NewApp(operService)
	logHandler = slog.New(logger.NewLogHandler(app.Logs, loggingOpts))
	slog.SetDefault(logHandler)
	app.Run()
}
