//go:build linux
// +build linux

package tun

import (
	"gvisor.dev/gvisor/pkg/rawfile"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/link/tun"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
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
