package netstack

import (
	"sync"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

func newTestPool(size int) *ConnPool {
	return NewConnPool(size)
}

func tcpTunConn() TunConn { return TunConn{Protocol: tcp.ProtocolNumber} }
func udpTunConn() TunConn { return TunConn{Protocol: udp.ProtocolNumber} }

func TestConnPool_Add_Get_Basic(t *testing.T) {
	t.Parallel()
	p := newTestPool(4)
	defer p.Close()

	if err := p.Add(tcpTunConn()); err != nil {
		t.Fatalf("Add: %v", err)
	}

	select {
	case got := <-p.Pool:
		if got.Protocol != tcp.ProtocolNumber {
			t.Errorf("got Protocol %d, want %d (TCP)", got.Protocol, tcp.ProtocolNumber)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for TunConn from pool")
	}
}

func TestConnPool_Add_ToClosedPool_ReturnsError(t *testing.T) {
	t.Parallel()
	p := newTestPool(4)
	p.Close()

	if err := p.Add(tcpTunConn()); err == nil {
		t.Error("Add to closed pool should return error, got nil")
	}
}

func TestConnPool_Close_Idempotent(t *testing.T) {
	t.Parallel()
	p := newTestPool(4)
	if err := p.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := p.Close(); err == nil {
		t.Error("second Close should return error (already closed), got nil")
	}
}

func TestConnPool_Closed_StateTransition(t *testing.T) {
	t.Parallel()
	p := newTestPool(4)
	if p.Closed() {
		t.Error("Closed() = true before Close(), want false")
	}
	p.Close()
	if !p.Closed() {
		t.Error("Closed() = false after Close(), want true")
	}
}

// TestConnPool_Get_UnblocksOnAdd verifies that a blocked Get() unblocks as soon
// as Add() delivers a connection. If Get() were to hold the pool mutex during the
// receive, Add() would never be able to send and both goroutines would deadlock.
func TestConnPool_Get_UnblocksOnAdd(t *testing.T) {
	t.Parallel()
	// NOTE: no defer p.Close() — if a deadlock is detected, Close() would also
	// block waiting for the same mutex that Get() holds.
	p := newTestPool(1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.Get() //nolint:errcheck
	}()

	time.Sleep(20 * time.Millisecond)

	added := make(chan error, 1)
	go func() {
		added <- p.Add(udpTunConn())
	}()

	select {
	case err := <-added:
		if err != nil {
			t.Logf("Add returned error: %v", err)
		}
		select {
		case <-done:
			p.Close() //nolint:errcheck
		case <-time.After(time.Second):
			t.Error("Get() did not unblock after Add() returned")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Add() blocked for 500ms — Get() is likely holding the mutex Add() needs")
	}
}

// TestConnPool_Add_DoesNotBlockUnderFullChannel verifies that Add() does not hold
// the pool mutex while the channel is full. If it does, the forwarder
// callbacks that call Add() under ns.Lock() will stall the whole netstack.
func TestConnPool_Add_DoesNotBlockUnderFullChannel(t *testing.T) {
	t.Parallel()
	// Pool with capacity 1.
	p := newTestPool(1)
	defer p.Close()

	// Fill the pool.
	if err := p.Add(tcpTunConn()); err != nil {
		t.Fatalf("pre-fill Add: %v", err)
	}

	// Start a goroutine that tries to Add to the full pool.
	// If Add holds the mutex while blocked on the channel, Closed() would deadlock.
	addDone := make(chan struct{})
	go func() {
		defer close(addDone)
		p.Add(udpTunConn()) //nolint:errcheck
	}()

	// Give the goroutine time to enter Add() and potentially acquire the lock.
	time.Sleep(20 * time.Millisecond)

	// While Add is blocked on the full channel (but should NOT hold the mutex),
	// Closed() should be able to acquire the mutex immediately.
	closedDone := make(chan bool, 1)
	go func() {
		closedDone <- p.Closed()
	}()

	select {
	case <-closedDone:
		// Closed() returned — mutex was not held by Add(). Good.
	case <-time.After(200 * time.Millisecond):
		t.Error("Closed() blocked for 200ms — Add() is holding the mutex " +
			"while blocked on a full channel")
	}

	// Drain the pool to let the blocked Add() complete.
	<-p.Pool
	<-addDone
}

// TestConnPool_Concurrent_AddClose verifies that concurrent Add and Close
// do not panic or deadlock. Run with -race.
func TestConnPool_Concurrent_AddClose(t *testing.T) {
	t.Parallel()
	p := newTestPool(8)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			conn := tcpTunConn()
			if i%2 == 0 {
				conn = udpTunConn()
			}
			p.Add(conn) //nolint:errcheck
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			select {
			case <-p.Pool:
			case <-p.CloseChan:
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		p.Close()
	}()

	waitDone := make(chan struct{})
	go func() { wg.Wait(); close(waitDone) }()

	select {
	case <-waitDone:
		// All goroutines finished cleanly.
	case <-time.After(2 * time.Second):
		t.Error("concurrent Add+Close deadlocked — " +
			"Add() likely holds the mutex that Close() needs")
	}
}

func TestConnPool_Pool_Channel_NotNil(t *testing.T) {
	t.Parallel()
	p := newTestPool(2)
	defer p.Close()

	if p.Pool == nil {
		t.Error("Pool channel is nil after NewConnPool")
	}
	if p.CloseChan == nil {
		t.Error("CloseChan is nil after NewConnPool")
	}
}
