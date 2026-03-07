GO_VER := 1.24.0
BLOAT_FILES := AUTHORS CONTRIBUTORS PATENTS VERSION favicon.ico robots.txt SECURITY.md CONTRIBUTING.md LICENSE README.md ./doc ./test ./api ./misc
GARBLE_VER := 0.14.2

GO ?= go

ARCH := $(shell uname -m)

ifeq ($(ARCH),aarch64)
    ARCH := arm64
else ifneq (,$(findstring armv5,$(ARCH)))
    ARCH := armv5
else ifneq (,$(findstring armv6,$(ARCH)))
    ARCH := armv6
else ifneq (,$(findstring armv7,$(ARCH)))
    ARCH := arm
else ifeq ($(ARCH),x86_64)
    ARCH := amd64
else ifeq ($(ARCH),x86)
    ARCH := 386
else ifeq ($(ARCH),i686)
    ARCH := 386
else ifeq ($(ARCH),i386)
    ARCH := 386
endif

TARGET_ARCH ?= $(ARCH)
TARGET_OS ?= linux

.PHONY: build
build: assets binaries

.PHONY: assets
assets: go agent

.PHONY: binaries
binaries: server client

.PHONY: go
go:
	# Build go
	cd artifacts && curl -L --output go/$(TARGET_OS)/$(TARGET_ARCH)/go.tar.gz https://dl.google.com/go/go$(GO_VER).$(TARGET_OS)-$(TARGET_ARCH).tar.gz
	cd artifacts/go/$(TARGET_OS)/$(TARGET_ARCH) && tar xvf go.tar.gz
	cd artifacts/go/$(TARGET_OS)/$(TARGET_ARCH) && rm -rf $(BLOAT_FILES)
	rm -f artifacts/go/$(TARGET_OS)/$(TARGET_ARCH)/go/pkg/tool/$(TARGET_OS)_$(TARGET_ARCH)/doc
	rm -f artifacts/go/$(TARGET_OS)/$(TARGET_ARCH)/go/pkg/tool/$(TARGET_OS)_$(TARGET_ARCH)/tour
	rm -f artifacts/go/$(TARGET_OS)/$(TARGET_ARCH)/go/pkg/tool/$(TARGET_OS)_$(TARGET_ARCH)/test2json
	# Build garble
	cd artifacts/go/$(TARGET_OS)/$(TARGET_ARCH)/go/bin && curl -L --output garble https://github.com/ttpreport/garble/releases/download/v$(GARBLE_VER)/garble_$(TARGET_OS)_$(TARGET_ARCH) && chmod +x garble
	# Bundle
	cd artifacts/go/$(TARGET_OS)/$(TARGET_ARCH) && zip -r go.zip ./go
	# Clean up
	cd artifacts/go/$(TARGET_OS)/$(TARGET_ARCH) && rm -rf go go.tar.gz

.PHONY: agent
agent:
	cd artifacts/agent && go mod tidy
	cd artifacts/agent && go mod vendor
	cd artifacts/agent && zip -r ../agent.zip .

.PHONY: server
server:
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath -o ligolo-mp ./cmd/server/

.PHONY: client
client:
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) CGO_ENABLED=0 $(GO) build -mod=vendor -trimpath -o ligolo-mp-client ./cmd/client/

.PHONY: protobuf
protobuf:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protobuf/ligolo.proto

.PHONY: clean
clean:
	rm -rf artifacts/agent.zip artifacts/go/*/*/*

.PHONY: install
install:
	./install_server.sh

.PHONY: service
service:
	./install_service.sh
