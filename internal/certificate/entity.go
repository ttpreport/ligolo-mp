package certificate

import (
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"errors"
)

type Certificate struct {
	Name        string
	Certificate []byte
	Key         []byte
	Thumbprint  [sha1.Size]byte
}

func (cert *Certificate) KeyPair() (tls.Certificate, error) {
	return tls.X509KeyPair(cert.Certificate, cert.Key)
}

func (cert *Certificate) CertPool() (*x509.CertPool, error) {
	certpool := x509.NewCertPool()
	if ok := certpool.AppendCertsFromPEM(cert.Certificate); !ok {
		return nil, errors.New("error parsing certificate")
	}

	return certpool, nil
}
