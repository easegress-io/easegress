/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	"net"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

var (
	appSubjectKeyID = []byte{102, 202, 33, 104, 96}
)

// MeshCertProvider is the EaseMesh in-memory, Self-Sign type cert provider.
type MeshCertProvider struct {
	RootCert     *spec.Certificate
	ServiceCerts map[string]*spec.Certificate
	mutex        sync.RWMutex
}

// NewMeshCertProvider creates a new mesh in-memory, self-sign cert provider
func NewMeshCertProvider() *MeshCertProvider {
	return &MeshCertProvider{
		ServiceCerts: make(map[string]*spec.Certificate),
	}
}

func uniqID(serviceName, IP string) string {
	return fmt.Sprintf("%s-%s", serviceName, IP)
}

// SignAppCertAndKey  Signs a cert, key pair for one service
func (mp *MeshCertProvider) SignAppCertAndKey(serviceName string, IP string, ttl time.Duration) (cert *spec.Certificate, err error) {
	if mp.RootCert == nil {
		err = fmt.Errorf("not root cert found")
		return
	}

	var (
		ca        *x509.Certificate
		caPrivKey *rsa.PrivateKey
	)
	ca, err = decodeCertPEM(mp.RootCert.CertBase64)
	if err != nil {
		logger.Errorf("decode root cert pem failed: %v", err)
		return
	}
	caPrivKey, err = decodeKeyPEM(mp.RootCert.KeyBase64)
	if err != nil {
		logger.Errorf("decode root key pem failed: %v", err)
		return
	}
	logger.Infof("try to sign serverName: %s, IP: %s, ttl: %s", serviceName, IP, ttl.String())
	now := time.Now()
	x509Cert := &x509.Certificate{
		SerialNumber: big.NewInt(defaultSerialNumber),
		Subject: pkix.Name{
			Country:      []string{defaultRootCertCountry},
			Locality:     []string{defaultRootCertLocality},
			Organization: []string{defaultRootCertOrganization},
		},
		NotBefore:    now,
		NotAfter:     now.Add(ttl),
		SubjectKeyId: appSubjectKeyID,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"*"},
		IPAddresses:  []net.IP{net.ParseIP(IP), net.IPv6loopback},
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		logger.Errorf("service: %s generate key failed: %v", serviceName, err)
		return
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, x509Cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		logger.Errorf("service: %s create cert failed: %v", serviceName, err)
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
		IP:          IP,
		CertBase64:  base64.StdEncoding.EncodeToString(certPEM.Bytes()),
		KeyBase64:   base64.StdEncoding.EncodeToString(certPrivKeyPEM.Bytes()),
		TTL:         ttl.String(),
		SignTime:    now.Format(time.RFC3339),
	}

	mp.SetAppCertAndKey(serviceName, IP, cert)
	return
}

func decodeKeyPEM(base64Key string) (*rsa.PrivateKey, error) {
	keyPEM, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, err
	}

	for len(keyPEM) > 0 {
		var block *pem.Block
		block, keyPEM = pem.Decode(keyPEM)
		if block == nil {
			return nil, fmt.Errorf("no key in pem")
		}
		if block.Type != typeKey || len(block.Headers) != 0 {
			continue
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return key, nil
	}

	return nil, fmt.Errorf("no key in pem")
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
			return nil, fmt.Errorf("no cert in pemM")
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
func (mp *MeshCertProvider) SignRootCertAndKey(ttl time.Duration) (cert *spec.Certificate, err error) {
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
		NotAfter:              now.Add(ttl),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		DNSNames:              []string{"*"},
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, defaultRsaBits)
	if err != nil {
		logger.Errorf("gen root's ca private keyfailed: %v", err)
		return
	}

	// Self-sign the root certificate
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		logger.Errorf("create root cert failed: %v", err)
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
		TTL:         ttl.String(),
		SignTime:    now.Format(time.RFC3339),
	}
	mp.SetRootCertAndKey(cert)
	return
}

// SetAppCertAndKey sets service cert into local memory
func (mp *MeshCertProvider) SetAppCertAndKey(serviceName, IP string, cert *spec.Certificate) error {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	mp.ServiceCerts[uniqID(serviceName, IP)] = cert
	return nil
}

// SetRootCertAndKey sets root cert into local memory
func (mp *MeshCertProvider) SetRootCertAndKey(cert *spec.Certificate) error {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	mp.RootCert = cert
	return nil
}

// GetAppCertAndKey get cert and key for one service
func (mp *MeshCertProvider) GetAppCertAndKey(serviceName, IP string) (cert *spec.Certificate, err error) {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	sCert, ok := mp.ServiceCerts[uniqID(serviceName, IP)]
	if ok {
		cert = sCert
	}
	err = fmt.Errorf("service :%s cert not found", uniqID(serviceName, IP))
	return
}

// GetRootCertAndKey get root ca cert and key
func (mp *MeshCertProvider) GetRootCertAndKey() (cert *spec.Certificate, err error) {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()
	if mp.RootCert != nil {
		cert = mp.RootCert
		return
	}
	err = fmt.Errorf("root cert not found")
	return
}

// ReleaseAppCertAndKey releases one service's cert and key
func (mp *MeshCertProvider) ReleaseAppCertAndKey(serviceName, IP string) error {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	_, ok := mp.ServiceCerts[uniqID(serviceName, IP)]
	if ok {
		delete(mp.ServiceCerts, uniqID(serviceName, IP))
	}
	return nil
}

// ReleaseRootCertAndKey releases root CA cert and key
func (mp *MeshCertProvider) ReleaseRootCertAndKey() error {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	mp.RootCert = nil
	return nil
}
