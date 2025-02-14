package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ttpreport/ligolo-mp/assets"
	"github.com/ttpreport/ligolo-mp/cmd/client/tui"
	"github.com/ttpreport/ligolo-mp/cmd/server/agents"
	"github.com/ttpreport/ligolo-mp/cmd/server/rpc"
	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/config"
	"github.com/ttpreport/ligolo-mp/internal/crl"
	"github.com/ttpreport/ligolo-mp/internal/operator"
	"github.com/ttpreport/ligolo-mp/internal/session"
	"github.com/ttpreport/ligolo-mp/internal/storage"
	"github.com/ttpreport/ligolo-mp/pkg/logger"
)

func main() {
	var daemon = flag.Bool("daemon", false, "enable daemon mode")
	var verbose = flag.Bool("v", false, "enable verbose mode")
	var listenInterface = flag.String("agent-addr", "0.0.0.0:11601", "listening address")
	var maxInflight = flag.Int("max-inflight", 4096, "max inflight TCP connections")
	var maxConnectionHandler = flag.Int("max-connection", 1024, "per tunnel connection pool size")
	var operatorAddr = flag.String("operator-addr", "0.0.0.0:58008", "Address for operators connections")
	var unpack = flag.Bool("unpack", false, "Unpack server files")

	flag.Parse()

	cfg := &config.Config{
		Environment:          "server",
		Verbose:              *verbose,
		ListenInterface:      *listenInterface,
		MaxInFlight:          *maxInflight,
		MaxConnectionHandler: *maxConnectionHandler,
		OperatorAddr:         *operatorAddr,
	}

	db, err := storage.New(cfg.GetStorageDir())
	if err != nil {
		panic(fmt.Sprintf("could not connect to storage: %v", err))
	}
	defer db.Close()

	certRepo, err := certificate.NewCertificateRepository(db)
	if err != nil {
		panic(err)
	}

	crlRepo, err := crl.NewCRLRepository(db)
	if err != nil {
		panic(err)
	}

	sessRepo, err := session.NewSessionRepository(db)
	if err != nil {
		panic(err)
	}

	operRepo, err := operator.NewOperatorRepository(db)
	if err != nil {
		panic(err)
	}

	crlService := crl.NewCRLService(crlRepo)
	certService := certificate.NewCertificateService(certRepo, crlService)
	sessService := session.NewSessionService(cfg, sessRepo)
	operService := operator.NewOperatorService(cfg, operRepo, certService)
	assetsService := assets.NewAssetsService(cfg)

	if err := certService.Init(); err != nil {
		panic(err)
	}

	operators, err := operService.AllOperators()
	if err != nil {
		panic(err)
	}

	if len(operators) < 1 {
		_, err := operService.NewOperator("admin", true, *operatorAddr)
		if err != nil {
			panic(err)
		}
	}

	if *unpack {
		if err := assetsService.Unpack(); err != nil {
			panic(err)
		}

		return
	}

	if err := sessService.CleanUp(); err != nil {
		panic(err)
	}

	var app *tui.App
	loggingOpts := &slog.HandlerOptions{}
	quit := make(chan error)

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

	if *daemon {
		logger := slog.New(slog.NewTextHandler(os.Stdout, loggingOpts))
		slog.SetDefault(logger)
	} else {
		app = tui.NewApp(operService)

		logger := slog.New(logger.NewLogHandler(app.Logs, loggingOpts))
		slog.SetDefault(logger)
	}

	go func() {
		quit <- agents.Run(cfg, certService, sessService)
	}()
	go func() {
		quit <- rpc.Run(cfg, certService, sessService, operService, assetsService)
	}()

	if *daemon {
		<-quit
	} else {
		app.Run()
	}

	slog.Info("Ligolo-mp instance terminated")
}
