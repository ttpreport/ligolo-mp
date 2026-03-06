package certificate

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"

	"github.com/ttpreport/ligolo-mp/v2/internal/crl"
	"github.com/ttpreport/ligolo-mp/v2/internal/storage"
)

func newTestCertService(t *testing.T) (*CertificateService, func()) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	certRepo, err := NewCertificateRepository(store)
	if err != nil {
		t.Fatalf("NewCertificateRepository: %v", err)
	}
	crlRepo, err := crl.NewCRLRepository(store)
	if err != nil {
		t.Fatalf("crl.NewCRLRepository: %v", err)
	}
	crlSvc := crl.NewCRLService(crlRepo)
	svc := NewCertificateService(certRepo, crlSvc)
	return svc, func() {
		store.Close()
		os.RemoveAll(dir)
	}
}

// --- CA generation ---

func TestGenerateCA_Produces_ValidPEM(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, err := svc.GenerateCA("test-ca")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	if len(ca.Certificate) == 0 {
		t.Error("CA Certificate PEM is empty")
	}
	if len(ca.Key) == 0 {
		t.Error("CA Key PEM is empty")
	}

	kp, err := tls.X509KeyPair(ca.Certificate, ca.Key)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	if kp.Leaf == nil {
		// Parse the cert so we can inspect it.
		kp.Leaf, err = x509.ParseCertificate(kp.Certificate[0])
		if err != nil {
			t.Fatalf("ParseCertificate: %v", err)
		}
	}
	if !kp.Leaf.IsCA {
		t.Error("Generated CA cert has IsCA=false")
	}
}

func TestGenerateCA_Thumbprint_NotZero(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, _ := svc.GenerateCA("tp-test")
	var zero [20]byte
	if ca.Thumbprint == zero {
		t.Error("Thumbprint is all-zero after GenerateCA")
	}
}

// --- leaf cert generation ---

func TestGenerateCert_SignedByCA(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, err := svc.GenerateCA("root")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	leaf, err := svc.GenerateCert("operator-bob", ca)
	if err != nil {
		t.Fatalf("GenerateCert: %v", err)
	}

	// Verify leaf signed by CA.
	caPool, err := ca.CertPool()
	if err != nil {
		t.Fatalf("CertPool: %v", err)
	}

	kp, err := tls.X509KeyPair(leaf.Certificate, leaf.Key)
	if err != nil {
		t.Fatalf("leaf X509KeyPair: %v", err)
	}
	parsed, err := x509.ParseCertificate(kp.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate leaf: %v", err)
	}

	opts := x509.VerifyOptions{Roots: caPool}
	if _, err := parsed.Verify(opts); err != nil {
		t.Errorf("leaf cert does not verify against CA: %v", err)
	}
}

func TestGenerateCert_CommonName(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, _ := svc.GenerateCA("root")
	leaf, err := svc.GenerateCert("my-operator", ca)
	if err != nil {
		t.Fatalf("GenerateCert: %v", err)
	}

	kp, _ := tls.X509KeyPair(leaf.Certificate, leaf.Key)
	parsed, _ := x509.ParseCertificate(kp.Certificate[0])
	if parsed.Subject.CommonName != "my-operator" {
		t.Errorf("CN = %q, want %q", parsed.Subject.CommonName, "my-operator")
	}
}

// --- Certificate entity methods ---

func TestCertificate_KeyPair_RoundTrip(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, _ := svc.GenerateCA("ca")
	_, err := ca.KeyPair()
	if err != nil {
		t.Errorf("KeyPair: %v", err)
	}
}

func TestCertificate_CertPool_OK(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, _ := svc.GenerateCA("ca")
	pool, err := ca.CertPool()
	if err != nil {
		t.Fatalf("CertPool: %v", err)
	}
	if pool == nil {
		t.Error("CertPool returned nil pool")
	}
}

func TestCertificate_ExpiryDate_NotZero(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, _ := svc.GenerateCA("ca")
	if ca.ExpiryDate().IsZero() {
		t.Error("ExpiryDate is zero")
	}
}

func TestCertificate_String_NotEmpty(t *testing.T) {
	c := &Certificate{Name: "alice", caName: "root"}
	if c.String() == "" {
		t.Error("String() is empty")
	}
}

// --- Init: idempotent PKI bootstrap ---

func TestCertificateService_Init_Idempotent(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	if err := svc.Init(); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	ca1, err := svc.GetCA()
	if err != nil {
		t.Fatalf("GetCA after first Init: %v", err)
	}
	if ca1 == nil {
		t.Fatal("GetCA after Init returned nil")
	}

	// Second Init must not overwrite the CA.
	if err := svc.Init(); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	ca2, err := svc.GetCA()
	if err != nil {
		t.Fatalf("GetCA after second Init: %v", err)
	}
	if ca2 == nil {
		t.Fatal("GetCA after second Init returned nil")
	}
	if string(ca1.Certificate) != string(ca2.Certificate) {
		t.Error("CA certificate changed between two Init() calls — should be idempotent")
	}
}

func TestCertificateService_Init_CreatesServerCerts(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	svc.Init() //nolint:errcheck

	opCert, err := svc.GetOperatorServerCert()
	if err != nil {
		t.Fatalf("GetOperatorServerCert: %v", err)
	}
	if opCert == nil {
		t.Error("GetOperatorServerCert returned nil after Init")
	}
	agCert, err := svc.GetAgentServerCert()
	if err != nil {
		t.Fatalf("GetAgentServerCert: %v", err)
	}
	if agCert == nil {
		t.Error("GetAgentServerCert returned nil after Init")
	}
}

// --- Revoke / IsRevoked ---

func TestCertificateService_Revoke_ThenIsRevoked(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, _ := svc.GenerateCA("root")
	leaf, _ := svc.GenerateCert("victim", ca)

	if err := svc.Revoke(leaf, "compromised"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// Parse the leaf to get a *x509.Certificate for IsRevoked.
	kp, _ := tls.X509KeyPair(leaf.Certificate, leaf.Key)
	parsed, _ := x509.ParseCertificate(kp.Certificate[0])

	if !svc.IsRevoked(parsed) {
		t.Error("IsRevoked = false after Revoke, want true")
	}
}

func TestCertificateService_NotRevoked_IsRevoked_False(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, _ := svc.GenerateCA("root")
	leaf, _ := svc.GenerateCert("clean", ca)

	kp, _ := tls.X509KeyPair(leaf.Certificate, leaf.Key)
	parsed, _ := x509.ParseCertificate(kp.Certificate[0])

	if svc.IsRevoked(parsed) {
		t.Error("IsRevoked = true for un-revoked cert, want false")
	}
}

// --- repository Save / GetOne / Remove ---

func TestCertificateRepository_Save_GetOne_Remove(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	defer func() { store.Close(); os.RemoveAll(dir) }()

	repo, err := NewCertificateRepository(store)
	if err != nil {
		t.Fatalf("NewCertificateRepository: %v", err)
	}

	cert := &Certificate{Name: "test-cert", Certificate: []byte("pem"), Key: []byte("key")}
	if err := repo.Save(cert); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.GetOne("test-cert")
	if err != nil {
		t.Fatalf("GetOne: %v", err)
	}
	if got == nil {
		t.Fatal("GetOne after Save returned nil")
	}
	if string(got.Certificate) != "pem" {
		t.Errorf("Certificate = %q, want %q", got.Certificate, "pem")
	}

	repo.Remove("test-cert") //nolint:errcheck
	afterRemove, err := repo.GetOne("test-cert")
	if err != nil {
		t.Fatalf("GetOne after Remove: %v", err)
	}
	if afterRemove != nil {
		t.Error("GetOne after Remove returned non-nil")
	}
}

// --- Proto round-trip ---

func TestCertificate_Proto_RoundTrip(t *testing.T) {
	svc, cleanup := newTestCertService(t)
	defer cleanup()

	ca, _ := svc.GenerateCA("root")
	leaf, _ := svc.GenerateCert("proto-test", ca)

	pb := leaf.Proto()
	if pb == nil {
		t.Fatal("Proto() returned nil")
	}
	if pb.Name != "proto-test" {
		t.Errorf("Proto Name = %q, want %q", pb.Name, "proto-test")
	}

	restored := ProtoToCertificate(pb)
	if string(restored.Certificate) != string(leaf.Certificate) {
		t.Error("Certificate bytes differ after Proto round-trip")
	}
}
