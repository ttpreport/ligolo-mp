package crl

import (
	"crypto/sha1"
)

type CRLService struct {
	repo *CRLRepository
}

func NewCRLService(repo *CRLRepository) *CRLService {
	return &CRLService{
		repo: repo,
	}
}

func (cs *CRLService) Revoke(thumbprint [sha1.Size]byte, reason string) error {
	return cs.repo.Save(&RevokedCertificate{
		Thumbprint: thumbprint,
		Reason:     reason,
	})
}

func (cs *CRLService) IsRevoked(thumbprint [sha1.Size]byte) bool {
	crlItem := &RevokedCertificate{
		Thumbprint: thumbprint,
	}

	if cs.repo.GetOne(crlItem.Hash()) != nil {
		return true
	}

	return false
}
