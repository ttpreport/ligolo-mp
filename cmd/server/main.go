package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ttpreport/ligolo-mp/assets"
	"github.com/ttpreport/ligolo-mp/cmd/server/agents"
	"github.com/ttpreport/ligolo-mp/cmd/server/rpc"
	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/config"
	"github.com/ttpreport/ligolo-mp/internal/crl"
	"github.com/ttpreport/ligolo-mp/internal/operator"
	"github.com/ttpreport/ligolo-mp/internal/session"
	"github.com/ttpreport/ligolo-mp/internal/storage"
)

func main() {
	var verboseFlag = flag.Bool("v", false, "enable verbose mode")
	var listenInterface = flag.String("agent-addr", "0.0.0.0:11601", "listening address")
	var maxInflight = flag.Int("max-inflight", 4096, "max inflight TCP connections")
	var maxConnectionHandler = flag.Int("max-connection", 4096, "per tunnel connection pool size")
	var operatorAddr = flag.String("operator-addr", "0.0.0.0:58008", "Address for operators connections")
	var unpack = flag.Bool("unpack", false, "Unpack server files")
	var initOperators = flag.Bool("init-operators", false, "Initialize operators (creates a first admin if none exists)")

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

	certRepo, err := certificate.NewCertificateRepository(storage)
	if err != nil {
		panic(err)
	}

	crlRepo, err := crl.NewCRLRepository(storage)
	if err != nil {
		panic(err)
	}

	sessRepo, err := session.NewSessionRepository(storage)
	if err != nil {
		panic(err)
	}

	operRepo, err := operator.NewOperatorRepository(storage)
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

	if *initOperators {
		operators, err := operService.AllOperators()
		if err != nil {
			panic(err)
		}

		if len(operators) < 1 {
			admin, err := operService.NewOperator("admin", true, *operatorAddr)
			if err != nil {
				panic(err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				panic(err)
			}

			path, err := admin.ToFile(cwd)
			if err != nil {
				panic(err)
			}

			fmt.Printf("Admin's config saved to: %s", path)

			return
		}

		fmt.Printf("First admin account already initialized")

		return
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

	quit := make(chan error, 1)

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
}
