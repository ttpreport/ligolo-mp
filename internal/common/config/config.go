package config

import (
	"os"
	"os/user"
	"path/filepath"
)

type Config struct {
	Verbose              bool
	ListenInterface      string
	MaxInFlight          int
	MaxConnectionHandler int
	OperatorAddr         string
}

func GetRootAppDir(suffix string) string {
	user, _ := user.Current()
	dir := filepath.Join(user.HomeDir, ".ligolo-mp-"+suffix)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			panic(err)
		}
	}
	return dir
}
