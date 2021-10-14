package certmanager

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

// MeshCertProvider is the EaseMesh Self-Sign type cert provider.
type MeshCertProvider struct {
	RootCert     *spec.Certificate
	RootCA       *x509.Certificate
	ServiceCerts map[string]*spec.Certificate

	rootCertRefreshInterval string
	rootValidationDuration  time.Duration

	appCertRefreshInterval string
	appValidationDuration  time.Duration
}

// NewMeshCertProvider will create a new mesh cert provider with existing certs and keys in
//  Etcd.
func NewMeshCertProvider(rootRefreshInterval, appRefreshInterval string) (*MeshCertProvider, error) {
	return nil, nil
}

// SignAppCertAndKey  Signs a cert, key pair for one service
func (mp *MeshCertProvider) SignAppCertAndKey(serviceName string, ttl time.Duration) (cert *spec.Certificate, err error) {
	if mp.RootCert == nil {
		err = fmt.Errorf("not root cert found")
		return
	}

	var ca *x509.Certificate
	ca, err = decodeCertPEM(mp.RootCert.CertBase64)
	if err != nil {
		return
	}
	now := time.Now()
	x509Cert := &x509.Certificate{
		SerialNumber: big.NewInt(defaultSerialNumber),
		Subject: pkix.Name{
			Country:      []string{defaultRootCertCountry},
			Locality:     []string{defaultRootCertLocality},
			Organization: []string{defaultRootCertOrganization},
		},
		//IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    now,
		NotAfter:     now.Add(mp.appValidationDuration),
		SubjectKeyId: []byte{102, 202, 33, 104, 96},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, x509Cert, ca, &certPrivKey.PublicKey, certPrivKey)
	if err != nil {
		return
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  typeCert,
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  typeKey,
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	cert = &spec.Certificate{
		ServiceName: serviceName,
		CertBase64:  base64.StdEncoding.EncodeToString(certPEM.Bytes()),
		KeyBase64:   base64.StdEncoding.EncodeToString(certPrivKeyPEM.Bytes()),
		TTL:         mp.appCertRefreshInterval,
		SignTime:    now.Format(time.RFC3339),
	}

	mp.ServiceCerts[serviceName] = cert
	return
}

func decodeCertPEM(base64Cert string) (*x509.Certificate, error) {
	certPEM, err := base64.StdEncoding.DecodeString(base64Cert)
	if err != nil {
		return nil, err
	}

	for len(certPEM) > 0 {
		var block *pem.Block
		block, certPEM = pem.Decode(certPEM)
		if block == nil {
			return nil, fmt.Errorf("NoCertificateInPEM")
		}
		if block.Type != typeCert || len(block.Headers) != 0 {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		return cert, nil
	}

	return nil, fmt.Errorf("no cert in pem")
}

// SignRootCertAndKey signs a cert, key pair for root.
func (mp *MeshCertProvider) SignRootCertAndKey(time.Duration) (cert *spec.Certificate, err error) {
	now := time.Now()
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(defaultSerialNumber),
		Subject: pkix.Name{
			CommonName:   spec.DefaultCommonName,
			Country:      []string{defaultRootCertCountry},
			Locality:     []string{defaultRootCertLocality},
			Organization: []string{defaultRootCertOrganization},
		},
		NotBefore:             now,
		NotAfter:              now.Add(mp.rootValidationDuration),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, defaultRsaBits)
	if err != nil {
		logger.Errorf("gen root's caPrivKey failed: %v", err)
		return
	}

	// Self-sign the root certificate
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return
	}

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  typeCert,
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  typeKey,
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	cert = &spec.Certificate{
		ServiceName: spec.DefaultCommonName,
		CertBase64:  base64.StdEncoding.EncodeToString(caPEM.Bytes()),
		KeyBase64:   base64.StdEncoding.EncodeToString(caPrivKeyPEM.Bytes()),
		TTL:         mp.rootCertRefreshInterval,
		SignTime:    now.Format(time.RFC3339),
	}
	mp.RootCert = cert
	return
}

// GetAppCertAndKey get cert and key for one service
func (mp *MeshCertProvider) GetAppCertAndKey(serviceName string) (cert *spec.Certificate, err error) {
	sCert, ok := mp.ServiceCerts[serviceName]
	if ok {
		cert = sCert
	}
	return nil, fmt.Errorf("service :%s cert not found", serviceName)
}

// GetRootCertAndKey get root ca cert and key
func (mp *MeshCertProvider) GetRootCertAndKey() (cert *spec.Certificate, err error) {
	if mp.RootCA != nil {
		cert = mp.RootCert
		return
	}
	err = fmt.Errorf("root cert not found")
	return
}

// ReleaseAppCertAndKey releases one service's cert and key
func (mp *MeshCertProvider) ReleaseAppCertAndKey(serviceName string) error {
	_, ok := mp.ServiceCerts[serviceName]
	if ok {
		delete(mp.ServiceCerts, serviceName)
	}
	return nil
}

// ReleaseRootCertAndKey releases root CA cert and key
func (mp *MeshCertProvider) ReleaseRootCertAndKey() error {
	mp.RootCA = nil
	return nil
}
