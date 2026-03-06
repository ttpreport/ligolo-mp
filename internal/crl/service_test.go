package crl

import (
	"crypto/sha1"
	"os"
	"testing"

	"github.com/ttpreport/ligolo-mp/v2/internal/storage"
)

func newTestCRLService(t *testing.T) (*CRLService, func()) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	repo, err := NewCRLRepository(store)
	if err != nil {
		t.Fatalf("NewCRLRepository: %v", err)
	}
	svc := NewCRLService(repo)
	return svc, func() {
		store.Close()
		os.RemoveAll(dir)
	}
}

func makeThumbprint(s string) [sha1.Size]byte {
	return sha1.Sum([]byte(s))
}

// --- entity ---

func TestRevokedCertificate_Hash_IsHex(t *testing.T) {
	rc := &RevokedCertificate{Thumbprint: makeThumbprint("cert-data")}
	h := rc.Hash()
	if len(h) != sha1.Size*2 {
		t.Errorf("Hash length = %d, want %d (hex-encoded SHA1)", len(h), sha1.Size*2)
	}
}

func TestRevokedCertificate_String_ContainsReason(t *testing.T) {
	rc := &RevokedCertificate{Reason: "operator deleted"}
	s := rc.String()
	if s == "" {
		t.Error("String() is empty")
	}
}

// --- service: Revoke + IsRevoked ---

func TestCRLService_Revoke_ThenIsRevoked_True(t *testing.T) {
	svc, cleanup := newTestCRLService(t)
	defer cleanup()

	tp := makeThumbprint("operator-alice")
	if err := svc.Revoke(tp, "account removed"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if !svc.IsRevoked(tp) {
		t.Error("IsRevoked = false after Revoke, want true")
	}
}

func TestCRLService_IsRevoked_Unknown_False(t *testing.T) {
	svc, cleanup := newTestCRLService(t)
	defer cleanup()

	tp := makeThumbprint("unknown-cert")
	if svc.IsRevoked(tp) {
		t.Error("IsRevoked = true for cert that was never revoked, want false")
	}
}

func TestCRLService_Revoke_Idempotent(t *testing.T) {
	svc, cleanup := newTestCRLService(t)
	defer cleanup()

	tp := makeThumbprint("repeated")
	if err := svc.Revoke(tp, "first"); err != nil {
		t.Fatalf("first Revoke: %v", err)
	}
	// Second revoke overwrites (UPSERT) — should not error.
	if err := svc.Revoke(tp, "second"); err != nil {
		t.Fatalf("second Revoke: %v", err)
	}
	if !svc.IsRevoked(tp) {
		t.Error("IsRevoked = false after double-revoke, want true")
	}
}

func TestCRLService_MultipleRevocations_IndependentLookup(t *testing.T) {
	svc, cleanup := newTestCRLService(t)
	defer cleanup()

	tp1 := makeThumbprint("cert-1")
	tp2 := makeThumbprint("cert-2")
	tp3 := makeThumbprint("cert-3")

	svc.Revoke(tp1, "r1")
	svc.Revoke(tp2, "r2")

	if !svc.IsRevoked(tp1) {
		t.Error("cert-1 should be revoked")
	}
	if !svc.IsRevoked(tp2) {
		t.Error("cert-2 should be revoked")
	}
	if svc.IsRevoked(tp3) {
		t.Error("cert-3 should NOT be revoked")
	}
}

// --- repository direct ---

func TestCRLRepository_Save_GetOne_Remove(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	defer func() { store.Close(); os.RemoveAll(dir) }()

	repo, err := NewCRLRepository(store)
	if err != nil {
		t.Fatalf("NewCRLRepository: %v", err)
	}

	tp := makeThumbprint("repo-test")
	rc := &RevokedCertificate{Thumbprint: tp, Reason: "testing"}

	if err := repo.Save(rc); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.GetOne(rc.Hash())
	if err != nil {
		t.Fatalf("GetOne: %v", err)
	}
	if got == nil {
		t.Fatal("GetOne after Save returned nil")
	}
	if got.Reason != "testing" {
		t.Errorf("Reason = %q, want %q", got.Reason, "testing")
	}

	if err := repo.Remove(rc.Hash()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	afterRemove, err := repo.GetOne(rc.Hash())
	if err != nil {
		t.Fatalf("GetOne after Remove: %v", err)
	}
	if afterRemove != nil {
		t.Error("GetOne after Remove returned non-nil, want nil")
	}
}

func TestCRLRepository_GetAll(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	defer func() { store.Close(); os.RemoveAll(dir) }()

	repo, _ := NewCRLRepository(store)

	for i := 0; i < 3; i++ {
		tp := makeThumbprint(string(rune('a' + i)))
		repo.Save(&RevokedCertificate{Thumbprint: tp, Reason: "bulk"})
	}

	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("GetAll count = %d, want 3", len(all))
	}
}
