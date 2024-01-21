package relay

import (
	"io"
	"net"
	"sync"
)

func relay(src net.Conn, dst net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()

	io.Copy(dst, src)
}

func StartRelay(src net.Conn, dst net.Conn) {
	defer src.Close()
	defer dst.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go relay(src, dst, &wg)
	go relay(dst, src, &wg)

	wg.Wait()
}
