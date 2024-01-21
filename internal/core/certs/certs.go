package certs

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/ttpreport/ligolo-mp/internal/core/storage"
)

func Init(storage *storage.Store) error {
	var err error
	var caCert, caKey []byte

	dbcacert, err := storage.GetCert("_ca_")
	if err != nil {
		return err
	}

	if dbcacert == nil {
		caCert, caKey, err = GenerateCA()
		if err != nil {
			return err
		}

		_, err = storage.AddCert("_ca_", caCert, caKey)
		if err != nil {
			return err
		}
	} else {
		caCert = dbcacert.Certificate
		caKey = dbcacert.Key
	}

	dbservercert, err := storage.GetCert("_server_")
	if err != nil {
		return err
	}

	if dbservercert == nil {
		serverCert, serverKey, err := GenerateCert("_server_", caCert, caKey)
		if err != nil {
			return err
		}

		_, err = storage.AddCert("_server_", serverCert, serverKey)
		if err != nil {
			return err
		}
	}

	dbagentcert, err := storage.GetCert("_agent_")
	if err != nil {
		return err
	}

	if dbagentcert == nil {
		agentCert, agentKey, err := GenerateCert("_agent_", caCert, caKey)
		if err != nil {
			return err
		}

		_, err = storage.AddCert("_agent_", agentCert, agentKey)
		if err != nil {
			return err
		}
	}

	return nil
}

func GenerateCA() ([]byte, []byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
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

	// create our private and public key
	caPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	caPrivKeyx509, err := x509.MarshalECPrivateKey(caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: caPrivKeyx509,
	})

	return caPEM.Bytes(), caPrivKeyPEM.Bytes(), nil
}

func GenerateCert(name string, caPEM []byte, caPrivKeyPEM []byte) ([]byte, []byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}

	caBytes, _ := pem.Decode(caPEM)
	ca, err := x509.ParseCertificate(caBytes.Bytes)
	if err != nil {
		return nil, nil, err
	}
	caPrivKeyBytes, _ := pem.Decode(caPrivKeyPEM)
	caPrivKey, err := x509.ParseECPrivateKey(caPrivKeyBytes.Bytes)
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	certPrivKeyx509, err := x509.MarshalECPrivateKey(certPrivKey)
	if err != nil {
		return nil, nil, err
	}
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: certPrivKeyx509,
	})

	return certPEM.Bytes(), certPrivKeyPEM.Bytes(), nil
}
