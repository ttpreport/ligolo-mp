package assets

import (
	"archive/zip"
	"bytes"
	"embed"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/rs/xid"
	"github.com/ttpreport/ligolo-mp/internal/config"
	"github.com/ttpreport/ligolo-mp/internal/gogo"
)

var (
	//go:embed artifacts/agent.zip artifacts/go.zip
	FS embed.FS
)

type AssetsService struct {
	config *config.Config
}

func NewAssetsService(cfg *config.Config) *AssetsService {
	return &AssetsService{
		config: cfg,
	}
}

func (assets *AssetsService) Init() error {
	a, err := FS.ReadFile("artifacts/go.zip")
	if err != nil {
		return err
	}

	_, err = unzipBuf(a, assets.config.GetRootAppDir())
	if err != nil {
		return err
	}

	return nil
}

func (assets *AssetsService) renderAgent(socksServer string, socksUser string, socksPass string, server string, CACert string, AgentCert string, AgentKey string) (string, error) {
	agentDir, err := assets.setupAgentDir()
	if err != nil {
		return "", err
	}

	srcDir := filepath.Join(agentDir, "src")

	a, err := FS.ReadFile("artifacts/agent.zip")
	if err != nil {
		return "", err
	}

	_, err = unzipBuf(a, srcDir)
	if err != nil {
		return "", err
	}

	t := template.New("agent.go")

	agentFile := filepath.Join(srcDir, "agent.go")
	t.ParseFiles(agentFile)

	var tpl bytes.Buffer
	data := struct {
		SocksServer string
		SocksUser   string
		SocksPass   string
		Server      string
		Retry       bool
		CACert      string
		AgentCert   string
		AgentKey    string
	}{
		SocksServer: socksServer,
		SocksUser:   socksUser,
		SocksPass:   socksPass,
		Server:      server,
		CACert:      CACert,
		AgentCert:   AgentCert,
		AgentKey:    AgentKey,
	}
	if err := t.Execute(&tpl, data); err != nil {
		return "", err
	}

	agentFilePath := filepath.Join(agentDir, "src", "agent.go")
	fileWriter, err := os.OpenFile(agentFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		return "", err
	}

	_, err = tpl.WriteTo(fileWriter)
	return agentDir, err
}

func (assets *AssetsService) setupAgentDir() (string, error) {
	agentDir := filepath.Join(assets.config.GetRootAppDir(), "agents", xid.New().String())

	if _, err := os.Stat(agentDir); os.IsNotExist(err) {
		if err = os.MkdirAll(agentDir, 0700); err != nil {
			return "", err
		}

		srcDir := filepath.Join(agentDir, "src")
		if err = os.MkdirAll(srcDir, 0700); err != nil {
			return "", err
		}

		binDir := filepath.Join(agentDir, "bin")
		if err = os.MkdirAll(binDir, 0700); err != nil {
			return "", err
		}

		return agentDir, nil
	}

	return "", nil
}

func (assets *AssetsService) CompileAgent(goos string, goarch string, obfuscate bool, socksServer string, socksUser string, socksPass string, server string, CACert string, AgentCert string, AgentKey string) ([]byte, error) {
	agentDir, err := assets.renderAgent(socksServer, socksUser, socksPass, server, CACert, AgentCert, AgentKey)
	if err != nil {
		return nil, err
	}

	goConfig := &gogo.GoConfig{
		CGO:        "0",
		GOOS:       goos,
		GOARCH:     goarch,
		GOROOT:     gogo.GetGoRootDir(assets.config.GetRootAppDir()),
		GOCACHE:    gogo.GetGoCache(assets.config.GetRootAppDir()),
		GOMODCACHE: gogo.GetGoModCache(assets.config.GetRootAppDir()),
		ProjectDir: agentDir,
		Obfuscate:  obfuscate,
		GOGARBLE:   "*",
	}

	var destination string
	if goos == "windows" {
		destination = filepath.Join(agentDir, "bin", "agent.exe")
	} else {
		destination = filepath.Join(agentDir, "bin", "agent")
	}

	_, err = gogo.GoBuild(*goConfig, filepath.Join(agentDir, "src"), destination)
	if err != nil {
		return nil, err
	}

	agentBytes, err := os.ReadFile(destination)
	if err != nil {
		return nil, err
	}

	if err = os.RemoveAll(agentDir); err != nil {
		return nil, err
	}

	return agentBytes, nil
}

func unzipBuf(src []byte, dest string) ([]string, error) {
	var filenames []string
	reader, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		return filenames, err
	}

	for _, file := range reader.File {

		rc, err := file.Open()
		if err != nil {
			return filenames, err
		}
		defer rc.Close()

		fPath := filepath.Join(dest, file.Name)
		filenames = append(filenames, fPath)

		if file.FileInfo().IsDir() {
			os.MkdirAll(fPath, 0700)
		} else {
			if err = os.MkdirAll(filepath.Dir(fPath), 0700); err != nil {
				return filenames, err
			}
			outFile, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return filenames, err
			}
			_, err = io.Copy(outFile, rc)
			outFile.Close()
			if err != nil {
				return filenames, err
			}
		}
	}
	return filenames, nil
}
