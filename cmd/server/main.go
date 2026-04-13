package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ttpreport/ligolo-mp/v2/cmd/client/tui"
	"github.com/ttpreport/ligolo-mp/v2/cmd/server/agents"
	"github.com/ttpreport/ligolo-mp/v2/cmd/server/rpc"
	"github.com/ttpreport/ligolo-mp/v2/internal/asset"
	"github.com/ttpreport/ligolo-mp/v2/internal/certificate"
	"github.com/ttpreport/ligolo-mp/v2/internal/config"
	"github.com/ttpreport/ligolo-mp/v2/internal/crl"
	"github.com/ttpreport/ligolo-mp/v2/internal/operator"
	"github.com/ttpreport/ligolo-mp/v2/internal/session"
	"github.com/ttpreport/ligolo-mp/v2/internal/storage"
	"github.com/ttpreport/ligolo-mp/v2/pkg/logger"
)

func main() {
	var daemon = flag.Bool("daemon", false, "enable daemon mode")
	var verbose = flag.Bool("v", false, "enable verbose mode")
	var listenInterface = flag.String("agent-addr", "0.0.0.0:11601", "listening address")
	var maxInflight = flag.Int("max-inflight", 4096, "max inflight TCP connections")
	var maxConnectionHandler = flag.Int("max-connection", 1024, "per tunnel connection pool size")
	var operatorAddr = flag.String("operator-addr", "0.0.0.0:58008", "Address for operators connections")
	var insecureAgents = flag.Bool("insecure-agents", false, "Disable certificate verification for agents (insecure!)")
	var regenCerts = flag.Bool("rotate-pki", false, "regenerate all certificates (CA + server certs)")

	flag.Parse()

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
		Environment:          "server",
		Verbose:              *verbose,
		ListenInterface:      *listenInterface,
		MaxInFlight:          *maxInflight,
		MaxConnectionHandler: *maxConnectionHandler,
		OperatorAddr:         *operatorAddr,
		InsecureAgents:       *insecureAgents,
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

	assetRepo, err := asset.NewAssetRepository(db)
	if err != nil {
		panic(err)
	}

	crlService := crl.NewCRLService(crlRepo)
	certService := certificate.NewCertificateService(certRepo, crlService)

	if *regenCerts {
		fmt.Println("Regenerating all certificates...")
		if err := certService.RegenerateAll(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Done. All server certificates have been regenerated.")
		fmt.Println("WARNING: All existing operator profiles are now invalid and must be re-exported.")
		os.Exit(0)
	}

	sessService := session.NewSessionService(cfg, sessRepo)
	operService := operator.NewOperatorService(cfg, operRepo, certService)
	assetService := asset.NewAssetsService(cfg, assetRepo)

	if err := assetService.Init(); err != nil {
		panic(err)
	}

	if err := certService.Init(); err != nil {
		panic(err)
	}

	if err := sessService.Init(); err != nil {
		panic(err)
	}

	if err := operService.Init(); err != nil {
		panic(err)
	}

	app := tui.NewApp(operService)

	if !*daemon {
		logHandler = slog.New(logger.NewLogHandler(app.Logs, loggingOpts))
		slog.SetDefault(logHandler)
	}

	quit := make(chan error)
	go func() {
		quit <- agents.Run(cfg, certService, sessService)
	}()
	go func() {
		quit <- rpc.Run(cfg, certService, sessService, operService, assetService)
	}()

	if *daemon {
		<-quit
	} else {
		app.Run()
	}
}
