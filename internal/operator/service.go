package operator

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/ttpreport/ligolo-mp/internal/certificate"
	"github.com/ttpreport/ligolo-mp/internal/config"
)

type OperatorService struct {
	config      *config.Config
	repo        *OperatorRepository
	certService *certificate.CertificateService
}

func NewOperatorService(cfg *config.Config, repo *OperatorRepository, certService *certificate.CertificateService) *OperatorService {
	return &OperatorService{
		config:      cfg,
		repo:        repo,
		certService: certService,
	}
}

func (service *OperatorService) Init() error {
	operators, err := service.repo.GetAll()
	if err != nil {
		return err
	}

	if len(operators) < 1 {
		admin, err := service.NewOperator("admin", true, service.config.OperatorAddr)
		if err != nil {
			return err
		}

		path, err := admin.ToFile(service.config.GetRootAppDir())
		if err != nil {
			return err
		}
		slog.Info("Administrative config saved", slog.Any("path", path))
	}

	return nil
}

func (service *OperatorService) NewOperator(name string, isAdmin bool, server string) (*Operator, error) {
	oper := &Operator{
		Name:    name,
		IsAdmin: isAdmin,
		Server:  server,
	}

	if service.repo.Exists(oper) {
		return nil, fmt.Errorf("operator '%s' already exists", name)
	}

	CA := service.certService.GetCA()
	oper.CA = CA.Certificate

	operCert, err := service.certService.GenerateCert(oper.Name, CA)
	if err != nil {
		return nil, err
	}

	oper.Cert = operCert

	if err := service.repo.Save(oper); err != nil {
		return nil, err
	}

	return oper, nil
}

func (service *OperatorService) OperatorByName(name string) (*Operator, error) {
	return service.repo.GetOne(name)
}

func (service *OperatorService) AllOperators() ([]*Operator, error) {
	return service.repo.GetAll()
}

func (service *OperatorService) NewOperatorFromFile(path string) (*Operator, error) {
	var oper *Operator

	operBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(operBytes, &oper); err != nil {
		return nil, err
	}

	try := 1
	name := oper.Name
	for service.repo.Exists(oper) {
		oper.Name = fmt.Sprintf("%s (%d)", name, try)
		try++
	}

	if err := service.repo.Save(oper); err != nil {
		return nil, err
	}

	return oper, nil
}

func (service *OperatorService) RemoveOperator(id string) (*Operator, error) {
	oper, err := service.repo.GetOne(id)
	if err != nil {
		return nil, err
	}

	if oper == nil {
		return nil, fmt.Errorf("operator '%s' not found", id)
	}

	return oper, service.repo.Remove(oper)
}
