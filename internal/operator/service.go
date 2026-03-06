package operator

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ttpreport/ligolo-mp/v2/internal/certificate"
	"github.com/ttpreport/ligolo-mp/v2/internal/config"
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
	operators, err := service.AllOperators()
	if err != nil {
		return err
	}

	if len(operators) < 1 {
		_, err := service.NewOperator("admin", true, service.config.OperatorAddr)
		if err != nil {
			return err
		}
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

	CA, err := service.certService.GetCA()
	if err != nil {
		return nil, err
	}
	if CA == nil {
		return nil, fmt.Errorf("CA certificate not found")
	}
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

func (service *OperatorService) RemoveOperator(name string) (*Operator, error) {
	oper, err := service.repo.GetOne(name)
	if err != nil {
		return nil, err
	}

	if oper == nil {
		return nil, fmt.Errorf("operator '%s' not found", name)
	}

	// Revoke first: if the delete fails after revocation, the operator is locked
	// out (cert invalid) but still visible in the DB — an admin can retry the
	// delete. The reverse order (delete then revoke) leaves a deleted operator
	// with a valid cert, which is the dangerous case.
	if err := service.certService.Revoke(oper.Cert, "removed by admin"); err != nil {
		return nil, err
	}
	return oper, service.repo.Remove(oper)
}

func (service *OperatorService) PromoteOperator(name string) (*Operator, error) {
	oper, err := service.repo.GetOne(name)
	if err != nil {
		return nil, err
	}

	if oper == nil {
		return nil, fmt.Errorf("operator '%s' not found", name)
	}

	oper.IsAdmin = true

	return oper, service.repo.Save(oper)
}

func (service *OperatorService) DemoteOperator(name string) (*Operator, error) {
	oper, err := service.repo.GetOne(name)
	if err != nil {
		return nil, err
	}

	if oper == nil {
		return nil, fmt.Errorf("operator '%s' not found", name)
	}

	oper.IsAdmin = false

	return oper, service.repo.Save(oper)
}
