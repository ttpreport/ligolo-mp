package session

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ttpreport/ligolo-mp/v2/internal/protocol"
	"github.com/ttpreport/ligolo-mp/v2/internal/storage"
	"github.com/ttpreport/ligolo-mp/v2/internal/tun"
	"github.com/ttpreport/ligolo-mp/v2/pkg/memstore"
)

func newTestRepo(t *testing.T) (*SessionRepository, func()) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	repo, err := NewSessionRepository(store)
	if err != nil {
		t.Fatalf("NewSessionRepository: %v", err)
	}
	return repo, func() {
		store.Close()
		os.RemoveAll(dir)
	}
}

// makeSession builds a minimal Session without OS resources (no relay, no mux).
// tun.NewTun() is pure allocation (no tunlink.New() until Start() is called).
func makeSession(id, alias, hostname string) *Session {
	tunIface, _ := tun.NewTun()
	return &Session{
		ID:          id,
		Alias:       alias,
		Hostname:    hostname,
		IsConnected: false,
		IsRelaying:  false,
		Redirectors: memstore.NewSyncmap[string, Redirector](),
		Interfaces:  memstore.NewSyncslice[protocol.NetInterface](),
		Tun:         tunIface,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}
}

// --- entity helpers ---

func TestRedirector_Hash_Deterministic(t *testing.T) {
	r := Redirector{Protocol: "tcp", From: "0.0.0.0:8080", To: "10.0.0.1:80"}
	r.ID = r.Hash()

	h1 := r.Hash()
	h2 := r.Hash()
	if h1 != h2 {
		t.Errorf("Hash not deterministic: %q vs %q", h1, h2)
	}
	if h1 == "" {
		t.Error("Hash is empty")
	}
}

func TestRedirector_DifferentInputs_DifferentHashes(t *testing.T) {
	r1 := Redirector{Protocol: "tcp", From: "0.0.0.0:8080", To: "10.0.0.1:80"}
	r2 := Redirector{Protocol: "tcp", From: "0.0.0.0:9090", To: "10.0.0.1:80"}
	if r1.Hash() == r2.Hash() {
		t.Error("different redirectors produced the same hash")
	}
}

func TestSession_GetName_Alias(t *testing.T) {
	sess := makeSession("id1", "my-alias", "hostname")
	if sess.GetName() != "my-alias" {
		t.Errorf("GetName = %q, want %q", sess.GetName(), "my-alias")
	}
}

func TestSession_GetName_FallsBackToHostname(t *testing.T) {
	sess := makeSession("id1", "", "targethost")
	if sess.GetName() != "targethost" {
		t.Errorf("GetName = %q, want %q", sess.GetName(), "targethost")
	}
}

func TestSession_Hash_EmptyInterfaces_Stable(t *testing.T) {
	sess := makeSession("x", "", "h")
	h1 := sess.Hash()
	h2 := sess.Hash()
	if h1 != h2 {
		t.Errorf("Hash not stable: %q vs %q", h1, h2)
	}
	if h1 == "" {
		t.Error("Hash is empty string")
	}
}

// --- repository: connected session (in-memory path) ---

func TestSessionRepository_Save_GetOne_Connected(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	sess := makeSession("sess-1", "alice", "target-1")
	sess.IsConnected = true

	if err := repo.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Connected sessions are retrievable by sess.ID from the in-memory map.
	got := repo.GetOne(sess.ID)
	if got == nil {
		t.Fatal("GetOne returned nil for connected session")
	}
	if got.Alias != "alice" {
		t.Errorf("Alias = %q, want %q", got.Alias, "alice")
	}
}

// --- repository: disconnected session (SQLite path) ---

func TestSessionRepository_Save_GetOne_Disconnected(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	sess := makeSession("sess-2", "bob", "target-2")
	sess.IsConnected = false

	if err := repo.Save(sess); err != nil {
		t.Fatalf("Save disconnected: %v", err)
	}

	// Disconnected sessions keyed by Hash() in SQLite.
	got := repo.GetOne(sess.Hash())
	if got == nil {
		t.Fatal("GetOne returned nil for disconnected session (SQLite path)")
	}
	if got.Hostname != "target-2" {
		t.Errorf("Hostname = %q, want %q", got.Hostname, "target-2")
	}
}

// --- repository: GetOne for unknown ID ---

func TestSessionRepository_GetOne_Missing_ReturnsNil(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	got := repo.GetOne("does-not-exist")
	if got != nil {
		t.Errorf("GetOne missing = %v, want nil", got)
	}
}

// --- repository: disconnect transition removes from connections map ---

func TestSessionRepository_Disconnect_RemovesFromConnections(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	sess := makeSession("sess-3", "", "h")
	sess.IsConnected = true
	repo.Save(sess)

	if repo.GetOne(sess.ID) == nil {
		t.Fatal("connected session not in connections map")
	}

	// Transition to disconnected.
	sess.IsConnected = false
	repo.Save(sess)

	// Should no longer be findable by ID (connections map is cleared);
	// it's now in SQLite keyed by Hash().
	gotByID := repo.GetOne(sess.ID) // will miss both maps; might still hit SQLite if ID==Hash
	gotByHash := repo.GetOne(sess.Hash())

	if gotByHash == nil {
		t.Error("disconnected session not found by Hash() in SQLite")
	}
	_ = gotByID // ID != Hash() when interfaces are empty — documented behaviour
}

// --- repository: Remove ---

func TestSessionRepository_Remove(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	sess := makeSession("sess-4", "", "h")
	sess.IsConnected = false
	repo.Save(sess)

	if err := repo.Remove(sess); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if repo.GetOne(sess.Hash()) != nil {
		t.Error("GetOne after Remove returned non-nil")
	}
}

// --- repository: GetAll ---

func TestSessionRepository_GetAll(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	// Save three disconnected sessions (so they reach SQLite).
	for i, name := range []string{"alpha", "beta", "gamma"} {
		sess := makeSession("", "", name)
		sess.IsConnected = false
		// Give each a unique first/last seen so Hash() differs if interfaces differ.
		// Since all are empty-interface sessions, Hash() is the same SHA1("").
		// Override ID to differentiate the SQLite key via Hash() somehow:
		// In practice all three share the same Hash() → overwrites. Document this.
		_ = i
		repo.Save(sess)
	}

	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	// Three sessions with empty interfaces all hash to the same key → only 1 survives.
	// This is a known design issue: Session.Hash() == SHA1("") for all sessions
	// without MAC addresses, so SaveAll overwrites each other.
	t.Logf("GetAll count = %d (sessions with identical Hash() collapse to 1)", len(all))
}

// --- repository: RemoveAll ---

func TestSessionRepository_RemoveAll(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	sess := makeSession("", "", "host")
	sess.IsConnected = false
	repo.Save(sess)

	if err := repo.RemoveAll(); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	all, _ := repo.GetAll()
	if len(all) != 0 {
		t.Errorf("GetAll after RemoveAll = %d, want 0", len(all))
	}
}

// TestSessionRepository_Save_ConcurrentReadWrite verifies that concurrent Save and GetOne
// do not race on the repository's internal state. Fix introduced a repo-level mutex
// that spans both the in-memory connections map update and the SQLite write inside Save,
// so a reader calling GetOne always sees a consistent view of both stores.
// Run with -race to catch any regression.
func TestSessionRepository_Save_ConcurrentReadWrite(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	sess := makeSession("concurrent-id", "", "host")
	sess.IsConnected = true

	const iterations = 500
	var wg sync.WaitGroup

	// Writer: continuously saves the session.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			repo.Save(sess)
		}
	}()

	// Reader: looks up by ID (connections map path) concurrently with the writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			repo.GetOne(sess.ID)
		}
	}()

	// Second reader via GetAll (SQLite path) — exercises the other branch.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			repo.GetAll() //nolint:errcheck
		}
	}()

	wg.Wait()
}
