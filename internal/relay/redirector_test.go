package relay

import (
	"net"
	"testing"
	"time"
)

// listenRandom starts a TCP listener on a random port and returns it.
func listenRandom(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return l
}

func TestNewLRedirector_CreatesListener(t *testing.T) {
	t.Parallel()
	rdr, err := NewLRedirector("test-id", "tcp", "127.0.0.1:0", "127.0.0.1:1")
	if err != nil {
		t.Fatalf("NewLRedirector: %v", err)
	}
	defer rdr.Close()

	if rdr.ID != "test-id" {
		t.Errorf("ID = %q, want test-id", rdr.ID)
	}
	if rdr.Listener == nil {
		t.Error("Listener is nil after creation")
	}
}

func TestRedirector_String(t *testing.T) {
	t.Parallel()
	rdr := Redirector{ID: "id1", Network: "tcp", From: ":9000", To: "10.0.0.1:22"}
	s := rdr.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

// TestRedirector_DialFail_ContinuesListening verifies that a failed outbound dial
// does not kill the listener — the redirector logs the error, closes the inbound
// conn, and continues accepting.
func TestRedirector_DialFail_ContinuesListening(t *testing.T) {
	t.Parallel()

	// Target that immediately refuses connections.
	refuser, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	targetAddr := refuser.Addr().String()
	refuser.Close() // close immediately so dials to it are refused

	rdr, err := NewLRedirector("dial-fail", "tcp", "127.0.0.1:0", targetAddr)
	if err != nil {
		t.Fatalf("NewLRedirector: %v", err)
	}
	defer rdr.Close()

	listenerAddr := rdr.Listener.Addr().String()
	go rdr.ListenAndRelay() //nolint:errcheck

	// Make a connection to the redirector — this triggers a dial to targetAddr
	// which will be refused.
	conn, err := net.DialTimeout("tcp", listenerAddr, time.Second)
	if err != nil {
		t.Fatalf("dial to redirector: %v", err)
	}
	conn.Close()

	// Give ListenAndRelay a moment to process the failed dial.
	time.Sleep(50 * time.Millisecond)

	// The listener must still be alive after the dial failure.
	conn2, err := net.DialTimeout("tcp", listenerAddr, time.Second)
	if err != nil {
		t.Error("after dial failure, redirector listener is dead — should still accept")
	} else {
		conn2.Close()
	}
}

// TestRedirector_ListenAndRelay_AcceptError_Returns verifies that ListenAndRelay
// returns an error when the listener itself is closed (the only fatal condition).
func TestRedirector_ListenAndRelay_AcceptError_Returns(t *testing.T) {
	t.Parallel()

	rdr, err := NewLRedirector("accept-err", "tcp", "127.0.0.1:0", "127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- rdr.ListenAndRelay()
	}()

	// Close the listener — Accept() will return an error, ListenAndRelay should exit.
	rdr.Close()

	select {
	case err := <-done:
		if err == nil {
			t.Error("ListenAndRelay should return error when listener is closed, got nil")
		}
	case <-time.After(time.Second):
		t.Error("ListenAndRelay did not return after listener was closed")
	}
}

// TestRedirector_EndToEnd_Relays verifies the full happy path: client connects
// to the redirector, the redirector connects to a real server, and data flows.
func TestRedirector_EndToEnd_Relays(t *testing.T) {
	t.Parallel()

	// Echo server.
	echoLis := listenRandom(t)
	defer echoLis.Close()
	go func() {
		for {
			conn, err := echoLis.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 64)
				n, _ := c.Read(buf)
				c.Write(buf[:n])
			}(conn)
		}
	}()

	rdr, err := NewLRedirector("e2e", "tcp", "127.0.0.1:0", echoLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer rdr.Close()
	go rdr.ListenAndRelay() //nolint:errcheck

	// Connect through the redirector.
	client, err := net.DialTimeout("tcp", rdr.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial redirector: %v", err)
	}
	defer client.Close()

	msg := []byte("hello")
	client.SetDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, len(msg))
	if _, err := client.Read(buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != string(msg) {
		t.Errorf("echo = %q, want %q", buf, msg)
	}
}
