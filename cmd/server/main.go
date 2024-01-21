package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ttpreport/ligolo-mp/cmd/server/cli"
	"github.com/ttpreport/ligolo-mp/cmd/server/rpc"
	"github.com/ttpreport/ligolo-mp/internal/common/config"
	"github.com/ttpreport/ligolo-mp/internal/core/agents"
	"github.com/ttpreport/ligolo-mp/internal/core/certs"
	"github.com/ttpreport/ligolo-mp/internal/core/proxy"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
	"github.com/ttpreport/ligolo-mp/internal/core/tuns"
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
		Verbose:              *verboseFlag,
		ListenInterface:      *listenInterface,
		MaxInFlight:          *maxInflight,
		MaxConnectionHandler: *maxConnectionHandler,
		OperatorAddr:         *operatorAddr,
	}

	storage, err := storage.New(config.GetRootAppDir("server"))

	if err != nil {
		panic(fmt.Sprintf("could not connect to storage: %v", err))
	}

	if err = storage.InitServer(); err != nil {
		panic(fmt.Sprintf("could not init storage: %v", err))
	}

	if err = certs.Init(storage); err != nil {
		panic(fmt.Sprintf("could not initialize certificates: %v", err))
	}

	if *verboseFlag {
		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelDebug)

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: lvl,
		}))

		slog.SetDefault(logger)
	}

	if *daemonFlag {
		tuns := tuns.New(storage)
		agents := agents.New(storage)
		proxyController := proxy.New(cfg, storage)

		go proxyController.ListenAndServe()
		go agents.WaitForConnections(cfg, &proxyController, tuns)

		tuns.Restore()

		rpc.Run(cfg, storage, tuns, agents)
	} else {
		cli.Run(cfg, storage)
	}
}
