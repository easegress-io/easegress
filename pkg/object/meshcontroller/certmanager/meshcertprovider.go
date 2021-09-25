package certmanager

import (
	"time"

	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
)

// MeshCertProvider is the EaseMesh Self-issue type cert provider.
type MeshCertProvider struct {
	RootCert     spec.Certificate
	ServiceCerts map[string]spec.Certificate
	store        storage.Storage
}

// NewMeshCertProvider will create a new mesh cert provider with existing certs and keys in
//  Etcd.
func NewMeshCertProvider(store storage.Storage) (*MeshCertProvider, error) {
	return nil, nil
}

// IssueAppCertAndKey  issues a cert, key pair for one service
func (mp *MeshCertProvider) IssueAppCertAndKey(serviceName string, ttl time.Duration) (cert spec.Certificate, err error) {
	return
}

// IssueRootCertAndKey issues a cert, key pair for root
func (mp *MeshCertProvider) IssueRootCertAndKey(time.Duration) (cert spec.Certificate, err error) {
	return
}

// GetAppCertAndKey get cert and key for one service
func (mp *MeshCertProvider) GetAppCertAndKey(serviceName string) (cert spec.Certificate, err error) {
	return
}

// GetRootCertAndKey get root ca cert and key
func (mp *MeshCertProvider) GetRootCertAndKey() (cert spec.Certificate, err error) {
	return
}

// ReleaseAppCertAndKey releases one service's cert and key
func (mp *MeshCertProvider) ReleaseAppCertAndKey(serviceName string) error {
	return nil
}

// ReleaseRootCertAndKey releases root CA cert and key
func (mp *MeshCertProvider) ReleaseRootCertAndKey() error {
	return nil
}
