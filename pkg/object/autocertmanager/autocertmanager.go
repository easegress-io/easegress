/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package autocertmanager provides AutoCertManager to manage certificates automatically.
package autocertmanager

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"golang.org/x/crypto/acme"
	"golang.org/x/net/idna"
)

const (
	// Category is the category of AutoCertManager.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of AutoCertManager.
	Kind = "AutoCertManager"
)

var (
	aliases = []string{
		"autocert",
		"autocerts",
		"autocertmanagers",
	}

	globalACM atomic.Value
)

func init() {
	supervisor.Register(&AutoCertManager{})
	api.RegisterObject(&api.APIResource{
		Category:    Category,
		Kind:        Kind,
		Name:        strings.ToLower(Kind),
		Aliases:     aliases,
		ValiateHook: validateHook,
	})
}

func validateHook(operationType api.OperationType, spec *supervisor.Spec) error {
	if operationType != api.OperationTypeCreate || spec.Kind() != Kind {
		return nil
	}

	acms := []string{}
	supervisor.GetGlobalSuper().WalkControllers(func(controller *supervisor.ObjectEntity) bool {
		if controller.Spec().Kind() == Kind {
			acms = append(acms, controller.Spec().Name())
		}
		return true
	})

	if len(acms) >= 1 {
		return fmt.Errorf("only one AutoCertManager is allowed, existed: %v", acms)
	}

	return nil
}

type (
	// AutoCertManager is the controller for Automated Certificate Management.
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
		DirectoryURL    string       `json:"directoryURL" jsonschema:"required,format=url"`
		Email           string       `json:"email" jsonschema:"required,format=email"`
		RenewBefore     string       `json:"renewBefore" jsonschema:"required,format=duration"`
		EnableHTTP01    bool         `json:"enableHTTP01"`
		EnableTLSALPN01 bool         `json:"enableTLSALPN01"`
		EnableDNS01     bool         `json:"enableDNS01"`
		Domains         []DomainSpec `json:"domains" jsonschema:"required"`
	}

	// DomainSpec is the automated certificate management spec for a domain.
	DomainSpec struct {
		Name        string            `json:"name" jsonschema:"required"`
		DNSProvider map[string]string `json:"dnsProvider,omitempty"`
	}

	// CertificateStatus is the certificate status of a domain.
	CertificateStatus struct {
		Name       string    `json:"name"`
		ExpireTime time.Time `json:"expireTime"`
	}

	// Status is the status of AutoCertManager.
	Status struct {
		Domains []CertificateStatus `json:"domains"`
	}
)

// Validate validates the spec of AutoCertManager.
func (spec *Spec) Validate() error {
	if !(spec.EnableHTTP01 || spec.EnableTLSALPN01 || spec.EnableDNS01) {
		return fmt.Errorf("at least one challenge type must be enabled")
	}

	for i := range spec.Domains {
		d := &spec.Domains[i]

		// convert to puny code to support Chinese or other unicode domain names.
		_, err := idna.Lookup.ToASCII(d.Name)
		if err != nil && d.Name[0] == '*' {
			_, err = idna.Lookup.ToASCII(d.Name[1:])
		}
		if err != nil {
			return fmt.Errorf("domain name contains invalid characters: %s", d.Name)
		}

		if d.Name[0] != '*' {
			continue
		}

		if !spec.EnableDNS01 {
			return fmt.Errorf("find wildcard domain name but DNS-01 challenge is disabled: %s", d.Name)
		}

		if _, err := newDNSProvider(d); err != nil {
			return fmt.Errorf("DNS provider configuration is invalid: %v", err)
		}
	}
	return nil
}

// Zone returns the zone the domain belongs to.
func (spec *DomainSpec) Zone() string {
	if spec.DNSProvider != nil {
		return spec.DNSProvider["zone"]
	}
	return ""
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
		DirectoryURL:    acme.LetsEncryptURL,
		RenewBefore:     "720h",
		EnableHTTP01:    true,
		EnableTLSALPN01: true,
		EnableDNS01:     true,
		Domains:         []DomainSpec{},
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

func (acm *AutoCertManager) findDomain(name string, exactMatch bool) *Domain {
	// compare one by one, if many domains are configured, we need to refactor
	// this function to improve performance.
	for i := range acm.domains {
		domain := &acm.domains[i]
		if name == domain.Name {
			return domain
		}
		if exactMatch || !domain.isWildcard() {
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

		// convert to puny code to support Chinese or other unicode domain names.
		name, err := idna.Lookup.ToASCII(spec.Name)
		if err != nil && spec.Name[0] == '*' {
			name, _ = idna.Lookup.ToASCII(spec.Name[1:])
			name = "*" + name
		}

		d := &acm.domains[i]
		d.DomainSpec = spec
		d.nameInPunyCode = name

		cert, err := acm.storage.getCert(d.nameInPunyCode)
		if err != nil {
			logger.Infof("failed to load certificate for domain %s: %v", spec.Name, err)
			continue
		}
		d.certificate.Store(cert)
	}

	go acm.run()
	go acm.watchCertificate()

	globalACM.Store(acm)
}

// Status returns the status of AutoCertManager.
func (acm *AutoCertManager) Status() *supervisor.Status {
	status := &Status{}
	for i := range acm.domains {
		d := &acm.domains[i]
		status.Domains = append(status.Domains, CertificateStatus{
			Name:       d.Name,
			ExpireTime: d.certExpireTime(),
		})
	}
	return &supervisor.Status{ObjectStatus: status}
}

// Close closes AutoCertManager.
func (acm *AutoCertManager) Close() {
	acm.cancel()

	globalACM.CompareAndSwap(acm, (*AutoCertManager)(nil))
}

func (acm *AutoCertManager) renew() bool {
	allSucc := true
	deadline := time.Now().Add(acm.renewBefore)

	for i := range acm.domains {
		// Try to avoid race conditions. We should only renew certificates from
		// the leader node. And because it takes some time for the renew process
		// to complete, we need to check this before each iteration.
		if !acm.super.Cluster().IsLeader() {
			break
		}

		d := &acm.domains[i]
		if d.certExpireTime().After(deadline) {
			continue
		}

		logger.Infof("begin renew certificate for domain %s", d.Name)
		if err := d.renewCert(acm); err == nil {
			logger.Infof("certificate for domain %s has been renewed", d.Name)
		} else {
			logger.Errorf("failed to renew certificate for domain %s: %v", d.Name, err)
			allSucc = false
		}
	}

	return allSucc
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

func (acm *AutoCertManager) watchCertificate() {
	onChange := func(name string, cert *tls.Certificate) {
		d := acm.findDomain(name, false)
		if d != nil {
			d.updateCert(cert)
		}
	}

	acm.storage.watchCertificate(acm.stopCtx, onChange)
}

func (acm *AutoCertManager) run() {
	for {
		if err := acm.createAcmeClient(); err == nil {
			break
		}
		select {
		case <-acm.stopCtx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}

	for {
		waitDuration := time.Hour
		if allSucc := acm.renew(); !allSucc {
			waitDuration = 10 * time.Minute
		}

		select {
		case <-acm.stopCtx.Done():
			return
		case <-time.After(waitDuration):
		}
	}
}

// GetCertificate handles the tls hello.
func (acm *AutoCertManager) GetCertificate(chi *tls.ClientHelloInfo, tokenOnly bool) (*tls.Certificate, error) {
	name := chi.ServerName
	if name == "" {
		return nil, fmt.Errorf("missing server name")
	}
	if !strings.Contains(strings.Trim(name, "."), ".") {
		return nil, fmt.Errorf("server name component count invalid")
	}
	name = strings.TrimSuffix(name, ".") // golang.org/issue/18114

	// this is a token cert requested for TLS-ALPN challenge.
	if len(chi.SupportedProtos) == 1 && chi.SupportedProtos[0] == acme.ALPNProto {
		if !acm.spec.EnableTLSALPN01 {
			return nil, fmt.Errorf("TLS-ALPN01 challenge is disabled")
		}
		return acm.storage.getTLSALPNCert(name)
	}

	if tokenOnly {
		return nil, fmt.Errorf("certificate does not exist")
	}

	domain := acm.findDomain(name, false)
	if domain == nil {
		// return nil error if the domain is not managed by the AutoCertManager, so that
		// the Go HTTP package could check the the static certificates configured in the
		// HTTP server spec.
		return nil, nil
	}

	cert := domain.cert()
	if cert == nil {
		return nil, fmt.Errorf("host %s has no certificate", name)
	}

	return cert, nil
}

// HandleHTTP01Challenge handles HTTP-01 challenge.
func (acm *AutoCertManager) HandleHTTP01Challenge(w http.ResponseWriter, r *http.Request) {
	if !acm.spec.EnableHTTP01 {
		http.Error(w, "HTTP01 challenge is disabled", http.StatusNotFound)
		return
	}

	domain := acm.findDomain(r.Host, true)
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

func GetGlobalAutoCertManager() (*AutoCertManager, bool) {
	value := globalACM.Load()
	if value == nil {
		return nil, false
	}

	acm := value.(*AutoCertManager)

	if acm == nil {
		return nil, false
	}

	return acm, true
}
