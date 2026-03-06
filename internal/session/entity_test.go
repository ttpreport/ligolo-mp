package session

import (
	"encoding/json"
	"testing"
)

// --- NewRedirector (disconnected) ---

// TestNewRedirector_Disconnected_AddsToMap verifies that calling NewRedirector on
// a disconnected session stores the redirector in memory without attempting any
// remote protocol call (which would require a live yamux session).
func TestNewRedirector_Disconnected_AddsToMap(t *testing.T) {
	t.Parallel()
	sess := makeSession("", "", "h")

	if err := sess.NewRedirector("tcp", "0.0.0.0:9000", "10.0.0.1:22"); err != nil {
		t.Fatalf("NewRedirector: %v", err)
	}

	all := sess.Redirectors.All()
	if len(all) != 1 {
		t.Fatalf("got %d redirectors, want 1", len(all))
	}
	for _, r := range all {
		if r.Protocol != "tcp" || r.From != "0.0.0.0:9000" || r.To != "10.0.0.1:22" {
			t.Errorf("stored redirector fields wrong: %+v", r)
		}
	}
}

// TestNewRedirector_Disconnected_SameConfig_Idempotent verifies that adding the
// same redirector configuration twice results in one entry (map key = content hash).
func TestNewRedirector_Disconnected_SameConfig_Idempotent(t *testing.T) {
	t.Parallel()
	sess := makeSession("", "", "h")
	sess.NewRedirector("tcp", ":9000", "10.0.0.1:22") //nolint:errcheck
	sess.NewRedirector("tcp", ":9000", "10.0.0.1:22") //nolint:errcheck

	if n := len(sess.Redirectors.All()); n != 1 {
		t.Errorf("same config added twice → got %d redirectors, want 1", n)
	}
}

// --- Write-before-verify: connected session with closed multiplex ---

// TestNewRedirector_Connected_RemoteFailure_NotAddedToMap verifies the
// write-before-verify ordering: if the remote call to the agent fails, the
// redirector must NOT be stored in the session's in-memory map.
func TestNewRedirector_Connected_RemoteFailure_NotAddedToMap(t *testing.T) {
	t.Parallel()
	sess := makeSession("", "", "h")
	sess.IsConnected = true
	// sess.Multiplex == nil → IsMultiplexOpen() == false →
	// remoteCreateRedirector returns "multiplex is disconnected" error.

	err := sess.NewRedirector("tcp", "0.0.0.0:9000", "10.0.0.1:22")
	if err == nil {
		t.Fatal("expected error when multiplex is nil, got nil")
	}
	if n := len(sess.Redirectors.All()); n != 0 {
		t.Errorf("redirector was stored despite remote failure: got %d, want 0", n)
	}
}

// --- RemoveRedirector (disconnected) ---

func TestRemoveRedirector_Disconnected_RemovesFromMap(t *testing.T) {
	t.Parallel()
	sess := makeSession("", "", "h")
	sess.NewRedirector("tcp", ":9000", "10.0.0.1:22") //nolint:errcheck

	var id string
	for id = range sess.Redirectors.All() {
		break
	}

	if err := sess.RemoveRedirector(id); err != nil {
		t.Fatalf("RemoveRedirector: %v", err)
	}
	if sess.Redirectors.Exists(id) {
		t.Error("redirector still present after RemoveRedirector")
	}
}

func TestRemoveRedirector_NonExistent_NoError(t *testing.T) {
	t.Parallel()
	sess := makeSession("", "", "h")
	if err := sess.RemoveRedirector("does-not-exist"); err != nil {
		t.Errorf("RemoveRedirector(nonexistent) on disconnected session: got error %v, want nil", err)
	}
}

// --- Copy ---

// TestCopy_TransfersRedirectorsAndAlias verifies that Copy() carries metadata
// (Alias, FirstSeen) and redirectors to a fresh disconnected session.
// This is the path taken during session restore from the database: the saved
// session is passed as the source, and the fresh (just-reconnected) session is
// the destination. Both are disconnected during the copy so no remote call fires.
func TestCopy_TransfersRedirectorsAndAlias(t *testing.T) {
	t.Parallel()
	src := makeSession("", "my-alias", "h")
	src.NewRedirector("tcp", "0.0.0.0:9000", "192.168.1.1:22") //nolint:errcheck
	src.NewRedirector("udp", "0.0.0.0:5353", "8.8.8.8:53")     //nolint:errcheck

	dst := makeSession("", "", "h")
	dst.Copy(src)

	if dst.Alias != "my-alias" {
		t.Errorf("Alias = %q, want my-alias", dst.Alias)
	}
	if !dst.FirstSeen.Equal(src.FirstSeen) {
		t.Errorf("FirstSeen not copied: got %v, want %v", dst.FirstSeen, src.FirstSeen)
	}

	got := dst.Redirectors.All()
	if len(got) != 2 {
		t.Fatalf("got %d redirectors after Copy, want 2", len(got))
	}
	for id := range src.Redirectors.All() {
		if !dst.Redirectors.Exists(id) {
			t.Errorf("redirector %q missing from destination after Copy", id)
		}
	}
}

// --- JSON persistence round-trip ---

// TestSession_Redirectors_JSONRoundTrip verifies that redirectors survive a
// json.Marshal → json.Unmarshal cycle. This is the mechanism used by the SQLite
// storage layer: sessions are persisted as JSON blobs. If Redirectors.Data does
// not round-trip correctly, redirectors are silently lost on server restart.
func TestSession_Redirectors_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	sess := makeSession("sess-id", "alias", "host")
	sess.NewRedirector("tcp", "0.0.0.0:9000", "192.168.1.1:22") //nolint:errcheck
	sess.NewRedirector("udp", "0.0.0.0:5353", "8.8.8.8:53")     //nolint:errcheck

	data, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var restored Session
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if restored.Redirectors == nil {
		t.Fatal("Redirectors is nil after JSON round-trip")
	}
	got := restored.Redirectors.All()
	if len(got) != 2 {
		t.Fatalf("got %d redirectors after round-trip, want 2", len(got))
	}
	for id, r := range sess.Redirectors.All() {
		r2, ok := got[id]
		if !ok {
			t.Errorf("redirector %q lost after round-trip", id)
			continue
		}
		if r.Protocol != r2.Protocol || r.From != r2.From || r.To != r2.To {
			t.Errorf("redirector %q fields changed after round-trip: got %+v, want %+v", id, r2, r)
		}
	}
}

// TestSession_Redirectors_JSONRoundTrip_Empty verifies that an empty Redirectors
// map survives JSON round-trip without nil-pointer panics.
func TestSession_Redirectors_JSONRoundTrip_Empty(t *testing.T) {
	t.Parallel()
	sess := makeSession("", "", "h")

	data, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var restored Session
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if restored.Redirectors == nil {
		t.Fatal("Redirectors is nil after round-trip of session with no redirectors")
	}
	if n := len(restored.Redirectors.All()); n != 0 {
		t.Errorf("empty session round-trip produced %d redirectors, want 0", n)
	}
}
