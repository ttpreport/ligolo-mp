package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ttpreport/ligolo-mp/assets"
	"github.com/ttpreport/ligolo-mp/cmd/server/agents"
	"github.com/ttpreport/ligolo-mp/cmd/server/cli"
	"github.com/ttpreport/ligolo-mp/cmd/server/rpc"
	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/config"
	"github.com/ttpreport/ligolo-mp/internal/operator"
	"github.com/ttpreport/ligolo-mp/internal/session"
	"github.com/ttpreport/ligolo-mp/internal/storage"
)

func main() {
	var daemonFlag = flag.Bool("daemon", false, "enable daemon mode")
	var verboseFlag = flag.Bool("v", false, "enable verbose mode")
	var listenInterface = flag.String("agentaddr", "0.0.0.0:11601", "listening address")
	var maxInflight = flag.Int("maxinflight", 4096, "max inflight TCP connections")
	var maxConnectionHandler = flag.Int("maxconnection", 4096, "per tunnel connection pool size")
	var operatorAddr = flag.String("operatoraddr", "0.0.0.0:58008", "Address for operators connections")

	flag.Parse()

	cfg := &config.Config{
		Environment:          "server",
		Verbose:              *verboseFlag,
		ListenInterface:      *listenInterface,
		MaxInFlight:          *maxInflight,
		MaxConnectionHandler: *maxConnectionHandler,
		OperatorAddr:         *operatorAddr,
	}

	storage, err := storage.New(cfg.GetRootAppDir())

	if err != nil {
		panic(fmt.Sprintf("could not connect to storage: %v", err))
	}

	if *verboseFlag {
		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelDebug)

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: lvl,
		}))

		slog.SetDefault(logger)
	}

	certRepo, _ := certificate.NewCertificateRepository(storage)
	sessRepo, _ := session.NewSessionRepository(storage)
	operRepo, _ := operator.NewOperatorRepository(storage)

	certService := certificate.NewCertificateService(certRepo)
	sessService := session.NewSessionService(cfg, sessRepo)
	operService := operator.NewOperatorService(cfg, operRepo, certService)
	assetsService := assets.NewAssetsService(cfg)

	if err := certService.Init(); err != nil {
		panic(err)
	}

	if err := operService.Init(); err != nil {
		panic(err)
	}

	if err := sessService.CleanUp(); err != nil {
		panic(err)
	}

	if err := assetsService.Init(); err != nil {
		panic(err)
	}

	quit := make(chan error, 1)

	if *daemonFlag {
		go func() {
			quit <- agents.Run(cfg, certService, sessService)
		}()
		go func() {
			quit <- rpc.Run(cfg, certService, sessService, operService, assetsService)
		}()

		slog.Info("server started")

		ret := <-quit
		if ret != nil {
			slog.Info("server terminated", slog.Any("exit", ret))
		}
	} else {
		cli.Run(cfg, certService, operService)
	}
}
