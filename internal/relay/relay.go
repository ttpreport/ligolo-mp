package relay

import (
	"io"
	"net"
	"sync"
)

func StartRelay(src net.Conn, dst net.Conn) {
	done := make(chan struct{})
	once := &sync.Once{}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(dst, src)
		once.Do(func() { close(done) })
	}()

	go func() {
		defer wg.Done()
		io.Copy(src, dst)
		once.Do(func() { close(done) })
	}()

	<-done
	dst.Close()
	src.Close()

	wg.Wait()
}
