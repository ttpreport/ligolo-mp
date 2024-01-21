module github.com/ttpreport/ligolo-mp/internal/agent

go 1.21.3

replace github.com/ttpreport/ligolo-mp => ../../

require (
	github.com/hashicorp/yamux v0.1.0
	github.com/ttpreport/ligolo-mp v0.0.0-20230726084436-8bfe194b0948
	golang.org/x/net v0.20.0
)

require (
	github.com/go-ping/ping v1.1.0 // indirect
	github.com/google/uuid v1.4.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
)
