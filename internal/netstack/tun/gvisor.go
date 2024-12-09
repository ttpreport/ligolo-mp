//go:build linux
// +build linux

package tun

import (
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/link/fdbased"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/link/rawfile"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/link/tun"
	"github.com/ttpreport/gvisor-ligolo/pkg/tcpip/stack"
)

func Open(tunName string) (stack.LinkEndpoint, int, error) {
	mtu, err := rawfile.GetMTU(tunName)
	if err != nil {
		return nil, 0, err
	}

	fd, err := tun.Open(tunName)
	if err != nil {
		return nil, 0, err
	}

	linkEP, err := fdbased.New(&fdbased.Options{FDs: []int{fd}, MTU: mtu})
	if err != nil {
		return nil, 0, err
	}

	return linkEP, fd, nil
}
