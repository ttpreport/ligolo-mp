package main

import (
	"flag"
	"fmt"

	"github.com/ttpreport/ligolo-mp/cmd/client/cli"
	"github.com/ttpreport/ligolo-mp/internal/common/config"
	"github.com/ttpreport/ligolo-mp/internal/common/operator"
	"github.com/ttpreport/ligolo-mp/internal/core/storage"
)

func main() {
	var credsFile = flag.String("import", "", "Path to credentials file to import")

	flag.Parse()

	storage, err := storage.New(config.GetRootAppDir("client"))
	if err != nil {
		panic(fmt.Sprintf("could not connect to storage: %v", err))
	}

	if err = storage.InitClient(); err != nil {
		panic(err)
	}

	if *credsFile == "" {
		cli.Run(storage)
	} else {
		creds, err := operator.LoadFromFile(*credsFile)
		if err != nil {
			panic(err)
		}

		if err = storage.AddCred(creds.Name, creds.Server, creds.CACert, creds.OperatorCert, creds.OperatorKey); err != nil {
			panic(err)
		}

		fmt.Println("Credentials successfully imported!")
	}
}
