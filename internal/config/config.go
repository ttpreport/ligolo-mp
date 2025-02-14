package config

import (
	"os"
	"os/user"
	"path"
	"path/filepath"

	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type Config struct {
	Environment          string
	Verbose              bool
	ListenInterface      string
	MaxInFlight          int
	MaxConnectionHandler int
	OperatorAddr         string
}

func (cfg *Config) GetRootAppDir() string {
	user, _ := user.Current()
	dir := filepath.Join(user.HomeDir, ".ligolo-mp-"+cfg.Environment)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			panic(err)
		}
	}
	return dir
}

func (cfg *Config) GetAssetsDir() string {
	dir := path.Join(cfg.GetRootAppDir(), "assets")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			panic(err)
		}
	}
	return dir
}

func (cfg *Config) GetStorageDir() string {
	dir := path.Join(cfg.GetRootAppDir(), "storage")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			panic(err)
		}
	}
	return dir
}

func (cfg *Config) Proto() *pb.Config {
	return &pb.Config{
		OperatorServer: cfg.OperatorAddr,
		AgentServer:    cfg.ListenInterface,
	}
}
