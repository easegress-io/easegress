package certmanager

import (
	"time"

	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
)

const (
	// DefaultCommonName is the name of root ca cert.
	DefaultCommonName = "mesh-root-ca"

	typeCert                    = "CERTIFICATE"
	typeKey                     = "RSA PRIVATE KEY"
	defaultRootCertCountry      = "cn"
	defaultRootCertLocality     = "beijing"
	defaultRootCertOrganization = "megaease"
	defaultRsaBits              = 2046
	defaultSerialNumber         = 202100
)

type (

	// CertManager manages the mesh-wide mTLS cert/keys's refreshing, storing into local Etcd.
	CertManager struct {
		RootCARefreshInterval time.Duration
		AppRefreshInterval    time.Duration
		Provider              CertProvider
		service               *service.Service
	}

	// CertProvider is the interface declaring the methods for the Certificate provider, such as
	//   easemesh-self-Sign, Valt, and so on.
	CertProvider interface {
		// SignAppCertAndKey  Signs a cert, key pair for one service
		SignAppCertAndKey(serviceName string, ttl time.Duration) (cert *spec.Certificate, err error)

		// SignRootCertAndKey Signs a cert, key pair for root
		SignRootCertAndKey(time.Duration) (cert *spec.Certificate, err error)

		// GetAppCertAndKey get cert and key for one service
		GetAppCertAndKey(serviceName string) (cert *spec.Certificate, err error)

		// GetRootCertAndKey get root ca cert and key
		GetRootCertAndKey() (cert *spec.Certificate, err error)

		// ReleaseAppCertAndKey releases one service's cert and key
		ReleaseAppCertAndKey(serviceName string) error

		// ReleaseRootCertAndKey releases root CA cert and key
		ReleaseRootCertAndKey() error
	}
)

// NewCertManager creates a initialed certmanager.
func NewCertManager(rootCARefreshInterval, appRefreshInterval string, store storage.Storage) *CertManager {
	return nil
}

// RefreshRootCertAndKey refreshes the root ca cert/key
func (cm *CertManager) RefreshRootCertAndKey(done chan struct{}) (cert spec.Certificate, err error) {
	// check whether root cert/key need to be updated

	// if so, also update all system's service cert/key
	return
}

// RefreshingServicesCertAndKey refreshes all service's cert/key if they expires
func (cm *CertManager) RefreshingServicesCertAndKey(done chan struct{}) (cert spec.Certificate, err error) {

	// List all services

	// check one if needed to updated
	return
}

func (cm *CertManager) refreshRootCertAndKey() error {
	return nil
}

func (cm *CertManager) refreshService(serviceName string) error {
	return nil
}
