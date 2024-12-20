package certificate

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/ttpreport/ligolo-mp/internal/crl"
)

type CertificateService struct {
	caName           string
	operatorCertName string
	agentCertName    string
	repo             *CertificateRepository
	crl              *crl.CRLService
}

func NewCertificateService(repo *CertificateRepository, crl *crl.CRLService) *CertificateService {
	return &CertificateService{
		caName:           "__CA",
		operatorCertName: "__OPERATORS",
		agentCertName:    "__AGENTS",
		repo:             repo,
		crl:              crl,
	}
}

func (cs *CertificateService) GetCA() *Certificate {
	return cs.repo.GetOne(cs.caName)
}

func (cs *CertificateService) GetOperatorServerCert() *Certificate {
	return cs.repo.GetOne(cs.operatorCertName)
}

func (cs *CertificateService) GetAgentServerCert() *Certificate {
	return cs.repo.GetOne(cs.agentCertName)
}

func (cs *CertificateService) GetAll() ([]*Certificate, error) {
	return cs.repo.GetAll()
}

func (cs *CertificateService) Save(name string, cert *Certificate) error {
	return cs.repo.Save(cert)
}

func (cs *CertificateService) Remove(name string) error {
	return cs.repo.Remove(name)
}

func (cs *CertificateService) RegenerateCert(name string) (*Certificate, error) {
	err := cs.Remove(name)
	if err != nil {
		return nil, err
	}

	CACert := cs.repo.GetOne(cs.caName)
	if CACert == nil {
		return nil, fmt.Errorf("CA certificate not found")
	}

	cert, err := cs.GenerateCert(name, CACert)
	if err != nil {
		return nil, err
	}

	err = cs.Save(name, cert)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func (cs *CertificateService) Init() error {
	var err error
	var CAcert *Certificate

	CAcert = cs.GetCA()
	if CAcert == nil {
		CAcert, err = cs.GenerateCA(cs.caName)
		if err != nil {
			return err
		}

		err = cs.repo.Save(CAcert)
		if err != nil {
			return err
		}
	}

	operatorCert := cs.repo.GetOne(cs.operatorCertName)
	if operatorCert == nil {
		operatorCert, err := cs.GenerateCert(cs.operatorCertName, CAcert)
		if err != nil {
			return err
		}

		err = cs.repo.Save(operatorCert)
		if err != nil {
			return err
		}
	}

	agentCert := cs.repo.GetOne(cs.agentCertName)
	if agentCert == nil {
		agentCert, err := cs.GenerateCert(cs.agentCertName, CAcert)
		if err != nil {
			return err
		}

		err = cs.repo.Save(agentCert)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cs *CertificateService) GenerateCA(name string) (*Certificate, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	ca := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	caPrivKeyx509, err := x509.MarshalECPrivateKey(caPrivKey)
	if err != nil {
		return nil, err
	}
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: caPrivKeyx509,
	})

	cert := &Certificate{
		Name:        name,
		Certificate: caPEM.Bytes(),
		Key:         caPrivKeyPEM.Bytes(),
		Thumbprint:  cs.Thumbprint(caBytes),
	}

	return cert, nil
}

func (cs *CertificateService) GenerateCert(name string, CAcert *Certificate) (*Certificate, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: name,
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	caBytes, _ := pem.Decode(CAcert.Certificate)
	ca, err := x509.ParseCertificate(caBytes.Bytes)
	if err != nil {
		return nil, err
	}
	caPrivKeyBytes, _ := pem.Decode(CAcert.Key)
	caPrivKey, err := x509.ParseECPrivateKey(caPrivKeyBytes.Bytes)
	if err != nil {
		return nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	certPrivKeyx509, err := x509.MarshalECPrivateKey(certPrivKey)
	if err != nil {
		return nil, err
	}
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: certPrivKeyx509,
	})

	res := &Certificate{
		Name:        name,
		Certificate: certPEM.Bytes(),
		Key:         certPrivKeyPEM.Bytes(),
		Thumbprint:  cs.Thumbprint(certBytes),
	}

	return res, nil
}

func (cs *CertificateService) Thumbprint(rawCert []byte) [sha1.Size]byte {
	return sha1.Sum(rawCert)
}

func (cs *CertificateService) Revoke(cert *Certificate, reason string) error {
	return cs.crl.Revoke(cert.Thumbprint, reason)
}

func (cs *CertificateService) IsRevoked(cert *x509.Certificate) bool {
	return cs.crl.IsRevoked(cs.Thumbprint(cert.Raw))
}
