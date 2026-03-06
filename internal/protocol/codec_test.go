package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"
)

// roundTrip encodes then decodes a single envelope and returns the decoded payload.
func roundTrip(t *testing.T, env Envelope) interface{} {
	t.Helper()
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(env); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dec := NewDecoder(&buf)
	if err := dec.Decode(); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return dec.Envelope.Payload
}

func TestCodec_RoundTrip_AllPacketTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		msgType uint8
		payload interface{}
		check   func(t *testing.T, got interface{})
	}{
		{
			name:    "InfoRequest",
			msgType: MessageInfoRequest,
			payload: InfoRequestPacket{},
			check: func(t *testing.T, got interface{}) {
				if _, ok := got.(InfoRequestPacket); !ok {
					t.Errorf("got %T, want InfoRequestPacket", got)
				}
			},
		},
		{
			name:    "InfoReply",
			msgType: MessageInfoReply,
			payload: InfoReplyPacket{Name: "user@host", Hostname: "host"},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(InfoReplyPacket)
				if !ok {
					t.Fatalf("got %T, want InfoReplyPacket", got)
				}
				if p.Name != "user@host" {
					t.Errorf("Name = %q, want %q", p.Name, "user@host")
				}
				if p.Hostname != "host" {
					t.Errorf("Hostname = %q, want %q", p.Hostname, "host")
				}
			},
		},
		{
			name:    "ConnectRequest",
			msgType: MessageConnectRequest,
			payload: ConnectRequestPacket{Net: Networkv4, Transport: TransportTCP, Address: "10.0.0.1", Port: 8080},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(ConnectRequestPacket)
				if !ok {
					t.Fatalf("got %T, want ConnectRequestPacket", got)
				}
				if p.Address != "10.0.0.1" || p.Port != 8080 {
					t.Errorf("got Address=%q Port=%d, want 10.0.0.1:8080", p.Address, p.Port)
				}
			},
		},
		{
			name:    "ConnectResponse_Established",
			msgType: MessageConnectResponse,
			payload: ConnectResponsePacket{Established: true, Reset: false},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(ConnectResponsePacket)
				if !ok {
					t.Fatalf("got %T, want ConnectResponsePacket", got)
				}
				if !p.Established {
					t.Error("Established = false, want true")
				}
			},
		},
		{
			name:    "ConnectResponse_Reset",
			msgType: MessageConnectResponse,
			payload: ConnectResponsePacket{Established: false, Reset: true},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(ConnectResponsePacket)
				if !ok {
					t.Fatalf("got %T, want ConnectResponsePacket", got)
				}
				if !p.Reset {
					t.Error("Reset = false, want true")
				}
			},
		},
		{
			name:    "HostPingRequest",
			msgType: MessageHostPingRequest,
			payload: HostPingRequestPacket{Address: "192.168.1.1"},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(HostPingRequestPacket)
				if !ok {
					t.Fatalf("got %T, want HostPingRequestPacket", got)
				}
				if p.Address != "192.168.1.1" {
					t.Errorf("Address = %q, want 192.168.1.1", p.Address)
				}
			},
		},
		{
			name:    "HostPingResponse_Alive",
			msgType: MessageHostPingResponse,
			payload: HostPingResponsePacket{Alive: true},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(HostPingResponsePacket)
				if !ok {
					t.Fatalf("got %T, want HostPingResponsePacket", got)
				}
				if !p.Alive {
					t.Error("Alive = false, want true")
				}
			},
		},
		{
			name:    "RedirectorRequest",
			msgType: MessageRedirectorRequest,
			payload: RedirectorRequestPacket{ID: "rdr-1", Network: "tcp", From: ":9000", To: "10.0.0.2:22"},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(RedirectorRequestPacket)
				if !ok {
					t.Fatalf("got %T, want RedirectorRequestPacket", got)
				}
				if p.ID != "rdr-1" || p.To != "10.0.0.2:22" {
					t.Errorf("got ID=%q To=%q", p.ID, p.To)
				}
			},
		},
		{
			name:    "RedirectorResponse",
			msgType: MessageRedirectorResponse,
			payload: RedirectorResponsePacket{ID: "rdr-1", Err: false},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(RedirectorResponsePacket)
				if !ok {
					t.Fatalf("got %T, want RedirectorResponsePacket", got)
				}
				if p.ID != "rdr-1" {
					t.Errorf("ID = %q, want rdr-1", p.ID)
				}
			},
		},
		{
			name:    "RedirectorCloseRequest",
			msgType: MessageRedirectorCloseRequest,
			payload: RedirectorCloseRequestPacket{ID: "rdr-1"},
			check: func(t *testing.T, got interface{}) {
				p, ok := got.(RedirectorCloseRequestPacket)
				if !ok {
					t.Fatalf("got %T, want RedirectorCloseRequestPacket", got)
				}
				if p.ID != "rdr-1" {
					t.Errorf("ID = %q, want rdr-1", p.ID)
				}
			},
		},
		{
			name:    "RedirectorCloseResponse",
			msgType: MessageRedirectorCloseResponse,
			payload: RedirectorCloseResponsePacket{Err: false},
			check: func(t *testing.T, got interface{}) {
				if _, ok := got.(RedirectorCloseResponsePacket); !ok {
					t.Errorf("got %T, want RedirectorCloseResponsePacket", got)
				}
			},
		},
		{
			name:    "DisconnectRequest",
			msgType: MessageDisconnectRequest,
			payload: DisconnectRequestPacket{},
			check: func(t *testing.T, got interface{}) {
				if _, ok := got.(DisconnectRequestPacket); !ok {
					t.Errorf("got %T, want DisconnectRequestPacket", got)
				}
			},
		},
		{
			name:    "DisconnectResponse",
			msgType: MessageDisconnectResponse,
			payload: DisconnectResponsePacket{},
			check: func(t *testing.T, got interface{}) {
				if _, ok := got.(DisconnectResponsePacket); !ok {
					t.Errorf("got %T, want DisconnectResponsePacket", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := roundTrip(t, Envelope{Type: tc.msgType, Payload: tc.payload})
			tc.check(t, got)
		})
	}
}

// TestDecoder_MalformedGob_ReturnsError verifies that Decode() returns an error
// when the gob payload is malformed, rather than panicking.
func TestDecoder_MalformedGob_ReturnsError(t *testing.T) {
	t.Parallel()

	msgTypes := []uint8{
		MessageInfoRequest,
		MessageInfoReply,
		MessageConnectRequest,
		MessageConnectResponse,
		MessageHostPingRequest,
		MessageHostPingResponse,
		MessageRedirectorRequest,
		MessageRedirectorResponse,
		MessageRedirectorCloseRequest,
		MessageRedirectorCloseResponse,
		MessageDisconnectRequest,
		MessageDisconnectResponse,
	}

	for _, msgType := range msgTypes {
		msgType := msgType
		t.Run(fmt.Sprintf("type_%d", msgType), func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			buf.WriteByte(msgType)
			// size = 5 bytes of garbage
			if err := binary.Write(&buf, binary.LittleEndian, int32(5)); err != nil {
				t.Fatal(err)
			}
			buf.Write([]byte{0xFF, 0xFE, 0xFD, 0xFC, 0xFB}) // invalid gob

			dec := NewDecoder(&buf)

			panicked := false
			var decodeErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
					}
				}()
				decodeErr = dec.Decode()
			}()

			if panicked {
				t.Errorf("Decode panicked on malformed gob for message type %d; should return an error", msgType)
				return
			}
			if decodeErr == nil {
				t.Errorf("type %d: Decode should return an error for malformed payload, got nil", msgType)
			}
		})
	}
}

// TestDecoder_UnknownMessageType_ReturnsError verifies that an unknown message
// type byte returns an error rather than panicking.
func TestDecoder_UnknownMessageType_ReturnsError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.WriteByte(0xFF) // unknown type
	if err := binary.Write(&buf, binary.LittleEndian, int32(0)); err != nil {
		t.Fatal(err)
	}

	dec := NewDecoder(&buf)

	panicked := false
	var decodeErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		decodeErr = dec.Decode()
	}()

	if panicked {
		t.Error("Decode panicked on unknown message type; should return error")
	}
	if decodeErr == nil {
		t.Error("Decode should return error for unknown message type, got nil")
	}
}

// partialReader returns at most maxPerRead bytes per Read call, simulating a
// slow or fragmented network stream.
type partialReader struct {
	r          io.Reader
	maxPerRead int
}

func (p *partialReader) Read(buf []byte) (int, error) {
	if len(buf) > p.maxPerRead {
		buf = buf[:p.maxPerRead]
	}
	return p.r.Read(buf)
}

// TestDecoder_PartialRead_Corrupts verifies decoder behaviour when the underlying
// reader returns fewer bytes than the framed payload size (simulates a slow network).
// Using io.Reader.Read() instead of io.ReadFull() means only part of the gob payload
// may be consumed; Decode() must return an error or produce correct data — never
// silently produce wrong data.
func TestDecoder_PartialRead_Corrupts(t *testing.T) {
	t.Parallel()

	var encoded bytes.Buffer
	enc := NewEncoder(&encoded)
	if err := enc.Encode(Envelope{
		Type:    MessageConnectRequest,
		Payload: ConnectRequestPacket{Address: "10.1.2.3", Port: 443},
	}); err != nil {
		t.Fatal(err)
	}

	// Wrap in a partialReader that delivers only 1 byte per Read call.
	// binary.Read uses io.ReadFull internally and handles partial reads correctly.
	// The payload Read() call does not, so only 1 byte of the gob payload arrives.
	slow := &partialReader{r: &encoded, maxPerRead: 1}
	dec := NewDecoder(slow)

	decodeErr := dec.Decode()

	if decodeErr == nil {
		// No error: verify the payload is actually correct (full read somehow succeeded).
		p, ok := dec.Envelope.Payload.(ConnectRequestPacket)
		if !ok || p.Address != "10.1.2.3" || p.Port != 443 {
			t.Errorf("partial read produced wrong payload: got %+v", dec.Envelope.Payload)
		}
	}
}

// TestEncoder_Size_SetByEncoder verifies the encoder sets the Size field
// automatically when the caller leaves it zero.
func TestEncoder_Size_SetByEncoder(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(Envelope{
		Type:    MessageHostPingRequest,
		Payload: HostPingRequestPacket{Address: "1.2.3.4"},
	}); err != nil {
		t.Fatal(err)
	}

	// Read back the type byte and size field to confirm size > 0.
	var msgType uint8
	var size int32
	if err := binary.Read(&buf, binary.LittleEndian, &msgType); err != nil {
		t.Fatal(err)
	}
	if err := binary.Read(&buf, binary.LittleEndian, &size); err != nil {
		t.Fatal(err)
	}
	if size <= 0 {
		t.Errorf("encoder wrote Size = %d, want > 0", size)
	}
	if msgType != MessageHostPingRequest {
		t.Errorf("type byte = %d, want %d", msgType, MessageHostPingRequest)
	}
}
