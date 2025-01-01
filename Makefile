GO_VER = 1.22.2
BLOAT_FILES = AUTHORS CONTRIBUTORS PATENTS VERSION favicon.ico robots.txt SECURITY.md CONTRIBUTING.md LICENSE README.md ./doc ./test ./api ./misc
GARBLE_VER = 1.22.2

.PHONY: all
all: assets binaries

.PHONY: assets
assets: go agent

.PHONY: binaries
binaries: server client

.PHONY: go
go:
	# Build go
	cd assets/artifacts && curl -L --output go$(GO_VER).linux-amd64.tar.gz https://dl.google.com/go/go$(GO_VER).linux-amd64.tar.gz
	cd assets/artifacts && tar xvf go$(GO_VER).linux-amd64.tar.gz
	cd assets/artifacts/go && rm -rf $(BLOAT_FILES)
	rm -f assets/artifacts/go/pkg/tool/linux_amd64/doc
	rm -f assets/artifacts/go/pkg/tool/linux_amd64/tour
	rm -f assets/artifacts/go/pkg/tool/linux_amd64/test2json
	# Build garble
	cd assets/artifacts/go/bin && curl -L --output garble https://github.com/moloch--/garble/releases/download/v$(GARBLE_VER)/garble_linux && chmod +x garble
	# Bundle
	cd assets/artifacts && zip -r go.zip ./go
	# Clean up
	cd assets/artifacts && rm -rf go go$(GO_VER).linux-amd64.tar.gz

.PHONY: agent
agent:
	cd assets/agent && go mod tidy
	cd assets/agent && go mod vendor
	cd assets/agent && zip -r ../artifacts/agent.zip .

.PHONY: server
server:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -mod=vendor -trimpath -o ligolo-server ./cmd/server/

.PHONY: client
client:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -mod=vendor -trimpath -o ligolo-client_linux ./cmd/client/
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -mod=vendor -trimpath -o ligolo-client_windows ./cmd/client/

.PHONY: protobuf
protobuf:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protobuf/ligolo.proto

.PHONY: clean
clean:
	rm -rf assets/artifacts/agent.zip assets/artifacts/go.zip

.PHONY: install
install:
	./install.sh