package operator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ttpreport/ligolo-mp/internal/certificate"
	pb "github.com/ttpreport/ligolo-mp/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Operator struct {
	Name       string
	IsAdmin    bool
	IsOnline   bool `json:"-"`
	Server     string
	CA         []byte
	Cert       *certificate.Certificate
	Connection `json:"-"`
	client     pb.LigoloClient
}

type Connection struct {
	conn *grpc.ClientConn
}

func (oper *Operator) ToFile(path string) (string, error) {
	operBytes, err := json.Marshal(oper)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(path, fmt.Sprintf("%s_%s_ligolo-mp.json", oper.Name, oper.Server))
	if err = os.WriteFile(fullPath, operBytes, os.ModePerm); err != nil {
		return "", err
	}

	return fullPath, nil
}

func (oper *Operator) ToBytes() ([]byte, error) {
	operBytes, err := json.Marshal(oper)
	if err != nil {
		return nil, err
	}

	return operBytes, nil
}

func (oper *Operator) Connect() error {
	if oper.conn != nil {
		oper.Disconnect()
	}

	operatorCert, err := oper.Cert.KeyPair()
	if err != nil {
		return err
	}

	ca := x509.NewCertPool()
	if ok := ca.AppendCertsFromPEM(oper.CA); !ok {
		panic("failed to parse CACert")
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{operatorCert},
		RootCAs:            ca,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}

			options := x509.VerifyOptions{
				Roots: ca,
			}
			if options.Roots == nil {
				return errors.New("no root certificate")
			}
			if _, err := cert.Verify(options); err != nil {
				return err
			}

			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, oper.Server,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(20*1024*1024)),
		grpc.WithBlock(),
	)
	if err != nil {
		oper.conn = nil
		return err
	}

	oper.conn = conn
	oper.client = pb.NewLigoloClient(conn)

	return nil
}

func (oper *Operator) Disconnect() error {
	if err := oper.conn.Close(); err != nil {
		return err
	}

	oper.client = nil
	oper.conn = nil
	return nil
}

func (oper *Operator) IsConnected() bool {
	if oper.conn != nil {
		return true
	}

	return false
}

func (oper *Operator) Client() pb.LigoloClient {
	return oper.client
}

func (oper *Operator) Proto() *pb.Operator {
	return &pb.Operator{
		Name:     oper.Name,
		IsAdmin:  oper.IsAdmin,
		IsOnline: oper.IsOnline,
		Server:   oper.Server,
	}
}
