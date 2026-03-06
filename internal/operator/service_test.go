package operator_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/ttpreport/ligolo-mp/v2/internal/certificate"
	"github.com/ttpreport/ligolo-mp/v2/internal/config"
	"github.com/ttpreport/ligolo-mp/v2/internal/crl"
	"github.com/ttpreport/ligolo-mp/v2/internal/operator"
	"github.com/ttpreport/ligolo-mp/v2/internal/storage"
)

// buildStack assembles the full OperatorService dependency graph against a
// temporary SQLite database and returns the service and the db directory path.
func buildStack(t *testing.T) (*operator.OperatorService, string) {
	t.Helper()

	dir := t.TempDir()

	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	crlRepo, err := crl.NewCRLRepository(store)
	if err != nil {
		t.Fatalf("NewCRLRepository: %v", err)
	}
	crlSvc := crl.NewCRLService(crlRepo)

	certRepo, err := certificate.NewCertificateRepository(store)
	if err != nil {
		t.Fatalf("NewCertificateRepository: %v", err)
	}
	certSvc := certificate.NewCertificateService(certRepo, crlSvc)
	if err := certSvc.Init(); err != nil {
		t.Fatalf("certSvc.Init: %v", err)
	}

	operRepo, err := operator.NewOperatorRepository(store)
	if err != nil {
		t.Fatalf("NewOperatorRepository: %v", err)
	}

	cfg := &config.Config{OperatorAddr: "127.0.0.1:58008"}
	operSvc := operator.NewOperatorService(cfg, operRepo, certSvc)

	return operSvc, dir
}

// TestRemoveOperator_RevokeFailure_OperatorPreserved verifies the
// revoke-before-delete ordering invariant: if writing to the CRL fails,
// RemoveOperator must return an error and must NOT delete the operator row.
// A deleted-but-not-revoked operator is the dangerous failure mode; a
// revoke-failed-but-still-in-DB operator is safe because an admin can retry.
func TestRemoveOperator_RevokeFailure_OperatorPreserved(t *testing.T) {
	operSvc, dir := buildStack(t)

	if _, err := operSvc.NewOperator("alice", false, "127.0.0.1:58008"); err != nil {
		t.Fatalf("NewOperator: %v", err)
	}

	// Drop the CRL table via a second connection to force Revoke to fail.
	sabotage, err := sql.Open("sqlite", filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open sabotage connection: %v", err)
	}
	defer sabotage.Close()

	if _, err := sabotage.Exec("DROP TABLE crl"); err != nil {
		t.Fatalf("DROP TABLE crl: %v", err)
	}

	// RemoveOperator must propagate the CRL write error.
	if _, err := operSvc.RemoveOperator("alice"); err == nil {
		t.Fatal("RemoveOperator with broken CRL table: expected error, got nil")
	}

	// The operator row must still exist — the ordering invariant held.
	oper, err := operSvc.OperatorByName("alice")
	if err != nil {
		t.Fatalf("OperatorByName after failed RemoveOperator: %v", err)
	}
	if oper == nil {
		t.Error("operator 'alice' was deleted despite Revoke failure — ordering invariant violated")
	}
}
