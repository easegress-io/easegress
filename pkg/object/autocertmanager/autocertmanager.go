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

package autocertmanager

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"golang.org/x/crypto/acme"
	"golang.org/x/net/idna"
)

const (
	// Category is the category of AutoCertManager.
	// It is a business controller by now, but should be a system controller
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of AutoCertManager.
	Kind = "AutoCertManager"
)

type (
	//AutoCertManager is the controller for Automated Certificate Management
	AutoCertManager struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec
		storage   *storage
		client    *acme.Client

		stopCtx context.Context
		cancel  context.CancelFunc

		renewBefore time.Duration
		domains     []Domain
	}

	// Spec describes AutoCertManager.
	Spec struct {
		DirectoryURL    string       `yaml:"directoryUrl" jsonschema:"required,format=url"`
		Email           string       `yaml:"email" jsonschema:"required,format=email"`
		RenewBefore     string       `yaml:"renewBefore" jsonschema:"required,format=duration"`
		EnableHTTP01    bool         `yaml:"enableHTTP01"`
		EnableTLSALPN01 bool         `yaml:"enableTLSALPN01"`
		EnableDNS01     bool         `yaml:"enableDNS01"`
		Domains         []DomainSpec `yaml:"domains" jsonschema:"required"`
	}

	// DomainSpec is the automate certificate management spec for a domain
	DomainSpec struct {
		Name            string `yaml:"name" jsonschema:"required"`
		DNSProvider     string `yaml:"dnsProvider" jsonschema:"omitempty"`
		Zone            string `yaml:"zone" jsonschema:"omitempty"`
		APIToken        string `yaml:"apiToken" jsonschema:"omitempty"`
		AccessKeyID     string `yaml:"accessKeyId" jsonschema:"omitempty"`
		AccessKeySecret string `yaml:"accessKeySecret" jsonschema:"omitempty"`
	}

	// Status is the status of AutoCertManager
	Status struct {
	}
)

var (
	globalACM atomic.Value
)

func init() {
	supervisor.Register(&AutoCertManager{})
}

// Category returns the category of AutoCertManager.
func (acm *AutoCertManager) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of AutoCertManager.
func (acm *AutoCertManager) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of AutoCertManager.
func (acm *AutoCertManager) DefaultSpec() interface{} {
	return &Spec{
		RenewBefore:     "720h",
		EnableHTTP01:    true,
		EnableTLSALPN01: true,
		EnableDNS01:     true,
	}
}

// Init initializes AutoCertManager.
func (acm *AutoCertManager) Init(superSpec *supervisor.Spec) {
	acm.superSpec = superSpec
	acm.spec = superSpec.ObjectSpec().(*Spec)
	acm.super = superSpec.Super()

	acm.reload()
}

// Inherit inherits previous generation of AutoCertManager.
func (acm *AutoCertManager) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	acm.superSpec = superSpec
	acm.spec = superSpec.ObjectSpec().(*Spec)
	acm.super = superSpec.Super()

	acm.reload()
	previousGeneration.(*AutoCertManager).Close()
}

func (acm *AutoCertManager) findDomain(name string, wildcard bool) *Domain {
	for i := range acm.domains {
		domain := &acm.domains[i]
		if name == domain.Name {
			return domain
		}
		if !wildcard {
			continue
		}
		if !domain.isWildcard() {
			continue
		}
		if strings.HasSuffix(name, domain.Name[1:]) {
			return domain
		}
	}
	return nil
}

func (acm *AutoCertManager) reload() {
	acm.stopCtx, acm.cancel = context.WithCancel(context.Background())
	acm.storage = newStorage(acm.super.Cluster())

	acm.renewBefore, _ = time.ParseDuration(acm.spec.RenewBefore)

	acm.domains = make([]Domain, len(acm.spec.Domains))
	for i := range acm.spec.Domains {
		spec := &acm.spec.Domains[i]
		name, err := idna.Lookup.ToASCII(spec.Name)
		if err != nil && spec.Name[0] == '*' {
			name, err = idna.Lookup.ToASCII(spec.Name[1:])
			name = "*" + name
		}
		if err != nil {
			name = spec.Name
			logger.Warnf("domain name contains invalid characters: %s", name)
		}
		domain := &acm.domains[i]
		domain.DomainSpec = spec
		domain.nameInPunyCode = name

		cert, err := acm.storage.getCert(domain.nameInPunyCode)
		if err != nil {
			logger.Infof("failed to load certificate for domain %s: %v", spec.Name, err)
			continue
		}
		domain.certificate.Store(cert)
	}

	globalACM.Store(acm)
	go acm.run()
}

// Status returns the status of AutoCertManager.
func (acm *AutoCertManager) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: &Status{},
	}
}

// Close closes AutoCertManager.
func (acm *AutoCertManager) Close() {
	acm.cancel()
}

func (acm *AutoCertManager) renew() {
	deadline := time.Now().Add(acm.renewBefore)
	for i := range acm.domains {
		domain := &acm.domains[i]
		if domain.certificateState == CertStateRenewing {
			continue
		}
		if domain.certExpireTime().After(deadline) {
			continue
		}
		domain.setCertState(CertStateRenewing)
		go domain.renewCert(acm)
	}
}

func (acm *AutoCertManager) createAcmeClient() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		logger.Errorf("failed to generate new account: %v", err)
		return err
	}

	cl := &acme.Client{Key: key, DirectoryURL: acm.spec.DirectoryURL}
	acct := &acme.Account{Contact: []string{"mailto:" + acm.spec.Email}}
	if _, err := cl.Register(acm.stopCtx, acct, acme.AcceptTOS); err != nil {
		logger.Errorf("failed to register: %v", err)
		return err
	}

	acm.client = cl
	return nil
}

func (acm *AutoCertManager) run() {
	ticker := time.NewTicker(30 * time.Second)
	for {
		if err := acm.createAcmeClient(); err == nil {
			break
		}
		select {
		case <-acm.stopCtx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
		}
	}

	ticker.Reset(time.Hour)
	for {
		if acm.super.Cluster().IsLeader() {
			acm.renew()
		}
		select {
		case <-acm.stopCtx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
		}
	}
}

func (acm *AutoCertManager) getCertificate(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := chi.ServerName
	if name == "" {
		return nil, errors.New("missing server name")
	}
	if !strings.Contains(strings.Trim(name, "."), ".") {
		return nil, errors.New("server name component count invalid")
	}
	name = strings.TrimSuffix(name, ".") // golang.org/issue/18114

	// this is a token cert requested for TLS-ALPN challenge.
	if len(chi.SupportedProtos) == 1 && chi.SupportedProtos[0] == acme.ALPNProto {
		return acm.storage.getTLSALPNCert(name)
	}

	domain := acm.findDomain(name, true)
	if domain == nil {
		return nil, fmt.Errorf("host %s is not configured for auto cert", name)
	}

	cert := domain.cert()
	if cert == nil {
		return nil, fmt.Errorf("host %s has no certificate", name)
	}

	return cert, nil
}

func (acm *AutoCertManager) handleHTTP01Challenge(w http.ResponseWriter, r *http.Request) {
	domain := acm.findDomain(r.Host, false)
	if domain == nil {
		msg := fmt.Sprintf("host %q is not configured for auto cert", r.Host)
		http.Error(w, msg, http.StatusForbidden)
		return
	}

	data, err := acm.storage.getHTTPToken(r.Host, r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Write(data)
}

// GetCertificate implements tls.Config.GetCertificate
func GetCertificate(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
	p := globalACM.Load()
	if p == nil {
		return nil, fmt.Errorf("auto certificate manager is not started")
	}
	return p.(*AutoCertManager).getCertificate(chi)
}

// HandleHTTP01Challenge handles HTTP-01 challenge
func HandleHTTP01Challenge(w http.ResponseWriter, r *http.Request) {
	p := globalACM.Load()
	if p == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	p.(*AutoCertManager).handleHTTP01Challenge(w, r)
}
