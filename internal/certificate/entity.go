package certificate

import (
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"errors"

	pb "github.com/ttpreport/ligolo-mp/protobuf"
)

type Certificate struct {
	Name        string
	caName      string
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

func (cert *Certificate) Proto() *pb.Cert {
	keypair, err := cert.KeyPair()
	if err != nil {
		return nil
	}

	return &pb.Cert{
		Name:        cert.Name,
		ExpiryDate:  keypair.Leaf.NotAfter.String(),
		Certificate: cert.Certificate,
		Key:         cert.Key,
	}
}
