package relay

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// pipePair returns two connected net.Conn via net.Pipe().
func pipePair() (net.Conn, net.Conn) {
	return net.Pipe()
}

func TestStartRelay_BidirectionalCopy(t *testing.T) {
	t.Parallel()
	a, b := pipePair()

	// StartRelay(a, b): data written to a appears on b, and vice versa.
	go StartRelay(a, b)

	// Write from the "a" side (which StartRelay treats as src) to reach dst (b).
	// But since StartRelay calls io.Copy(dst, src) and io.Copy(src, dst),
	// writing to the underlying pipe end that StartRelay reads from will appear
	// at the other side.
	//
	// Use a separate pair to drive writes independently.
	src, srcPeer := pipePair()
	dst, dstPeer := pipePair()
	go StartRelay(srcPeer, dstPeer)

	msg := []byte("hello relay")
	go func() {
		src.Write(msg)
	}()

	buf := make([]byte, len(msg))
	dst.SetReadDeadline(time.Now().Add(time.Second))
	n, err := io.ReadFull(dst, buf)
	if err != nil {
		t.Fatalf("ReadFull: %v (read %d bytes)", err, n)
	}
	if !bytes.Equal(buf, msg) {
		t.Errorf("got %q, want %q", buf, msg)
	}

	src.Close()
	dst.Close()
}

func TestStartRelay_HalfClose_Propagates(t *testing.T) {
	t.Parallel()
	src, srcPeer := pipePair()
	dst, dstPeer := pipePair()

	go StartRelay(srcPeer, dstPeer)

	// Send data then close the write side.
	go func() {
		src.Write([]byte("ping"))
		src.Close()
	}()

	// The relay should copy "ping" to dst and then close dst when src closes.
	buf := make([]byte, 4)
	dst.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadFull(dst, buf); err != nil {
		t.Fatalf("reading data: %v", err)
	}
	if !bytes.Equal(buf, []byte("ping")) {
		t.Errorf("got %q, want ping", buf)
	}

	// After src closes, relay should close dst — subsequent read returns EOF/error.
	dst.SetReadDeadline(time.Now().Add(time.Second))
	n, err := dst.Read(buf)
	if n != 0 || err == nil {
		t.Errorf("expected EOF after relay closed dst, got n=%d err=%v", n, err)
	}
}

func TestStartRelay_LargePayload(t *testing.T) {
	t.Parallel()
	src, srcPeer := pipePair()
	dst, dstPeer := pipePair()

	go StartRelay(srcPeer, dstPeer)

	payload := bytes.Repeat([]byte("A"), 1<<16) // 64 KB
	go func() {
		io.Copy(src, bytes.NewReader(payload))
		src.Close()
	}()

	received, err := io.ReadAll(dst)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(received, payload) {
		t.Errorf("received %d bytes, want %d", len(received), len(payload))
	}
}
