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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"sync/atomic"
	"time"

	"github.com/libdns/libdns"
	"github.com/megaease/easegress/pkg/logger"
	"golang.org/x/crypto/acme"
)

// CertState is the state of a certificate
type CertState int32

const (
	// CertStateNormal means the certificate state is normal
	CertStateNormal = iota
	// CertStateRenewing means the certificate is under a renew process
	CertStateRenewing
)

// Domain represents a domain for automated certificate management
type Domain struct {
	*DomainSpec
	nameInPunyCode   string
	certificate      atomic.Value
	certificateState int32
	cleanup          []func() error
	ctx              context.Context
}

// isWildcard returns whether the domain is for a wildcard one
func (d *Domain) isWildcard() bool {
	return d.nameInPunyCode[0] == '*'
}

// cert returns the certificate for the domain
func (d *Domain) cert() *tls.Certificate {
	if x := d.certificate.Load(); x == nil {
		return nil
	} else {
		return x.(*tls.Certificate)
	}
}

func (d *Domain) certExpireTime() time.Time {
	cert := d.cert()
	if cert == nil {
		return time.Time{}
	}
	return cert.Leaf.NotAfter
}

func (d *Domain) certState() CertState {
	return CertState(atomic.LoadInt32(&d.certificateState))
}

func (d *Domain) setCertState(state CertState) {
	atomic.StoreInt32(&d.certificateState, int32(state))
}

func (d *Domain) runHTTP01(acm *AutoCertManager, chal *acme.Challenge) error {
	if !acm.spec.EnableHTTP01 {
		return nil
	}

	client := acm.client

	path := client.HTTP01ChallengePath(chal.Token)
	body, err := client.HTTP01ChallengeResponse(chal.Token)
	if err != nil {
		return err
	}

	err = acm.storage.putHTTPToken(d.nameInPunyCode, path, []byte(body))
	if err != nil {
		return err
	}

	d.cleanup = append(d.cleanup, func() error {
		return acm.storage.deleteHTTPToken(d.nameInPunyCode, path)
	})

	_, err = client.Accept(d.ctx, chal)
	return err
}

func (d *Domain) runTLSALPN01(acm *AutoCertManager, z *acme.Authorization, chal *acme.Challenge) error {
	if !acm.spec.EnableTLSALPN01 {
		return nil
	}

	client := acm.client

	cert, err := client.TLSALPN01ChallengeCert(chal.Token, z.Identifier.Value)
	if err != nil {
		return err
	}

	err = acm.storage.putTLSALPNCert(d.nameInPunyCode, &cert)
	if err != nil {
		return err
	}

	d.cleanup = append(d.cleanup, func() error {
		return nil
	})

	_, err = client.Accept(d.ctx, chal)
	return err
}

func (d *Domain) runDNS01(acm *AutoCertManager, chal *acme.Challenge) error {
	if !acm.spec.EnableDNS01 || d.DomainSpec.DNSProvider == "" {
		return nil
	}

	client := acm.client

	dp, err := newDNSProvider(d.DomainSpec)
	if err != nil {
		return err
	}

	value, err := client.DNS01ChallengeRecord(chal.Token)
	if err != nil {
		return err
	}

	records, err := dp.AppendRecords(d.ctx, d.Zone, []libdns.Record{{
		Type:  "TXT",
		Name:  "_acme-challenge",
		Value: value,
	}})

	if err != nil {
		return err
	}

	d.cleanup = append(d.cleanup, func() error {
		dp.DeleteRecords(d.ctx, d.Zone, records)
		return nil
	})

	// DNS challenge need more time to complete
	select {
	case <-time.After(90 * time.Second):
	case <-d.ctx.Done():
		return d.ctx.Err()
	}

	_, err = client.Accept(d.ctx, chal)
	return err
}

func newCSR(id acme.AuthzID) ([]byte, crypto.Signer, error) {
	var csr x509.CertificateRequest

	csr.DNSNames = append(csr.DNSNames, id.Value)
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	b, err := x509.CreateCertificateRequest(rand.Reader, &csr, k)
	if err != nil {
		return nil, nil, err
	}
	return b, k, nil
}

func (d *Domain) doRenewCert(acm *AutoCertManager) error {
	var zurls []string
	client := acm.client

	ctx, cancel := context.WithTimeout(acm.stopCtx, 10*time.Minute)
	d.ctx = ctx

	defer func() {
		for _, fn := range d.cleanup {
			fn()
		}
		d.cleanup = nil

		// Deactivate all authorizations we satisfied earlier.
		for _, v := range zurls {
			if err := client.RevokeAuthorization(ctx, v); err != nil {
				logger.Errorf("RevokeAuthorization(%q): %v", v, err)
				continue
			}
		}

		d.ctx = nil
		cancel()
	}()

	ids := acme.DomainIDs(d.nameInPunyCode)
	order, err := client.AuthorizeOrder(ctx, ids)
	if err != nil {
		logger.Errorf("AuthorizeOrder: %v", err)
		return err
	}

	for _, u := range order.AuthzURLs {
		z, err := client.GetAuthorization(ctx, u)
		if err != nil {
			logger.Errorf("GetAuthorization(%q): %v", u, err)
			return err
		}
		if z.Status != acme.StatusPending {
			continue
		}

		for _, chal := range z.Challenges {
			switch chal.Type {
			case "http-01":
				d.runHTTP01(acm, chal)
			case "dns-01":
				d.runDNS01(acm, chal)
			case "tls-alpn-01":
				d.runTLSALPN01(acm, z, chal)
			default:
				logger.Errorf("unknown challenge type %q", chal.Type)
			}
		}

		if _, err = client.WaitAuthorization(ctx, z.URI); err != nil {
			logger.Errorf("WaitAuthorization(%q): %v", z.URI, err)
			return err
		}
		zurls = append(zurls, u)
	}

	if _, err := client.WaitOrder(ctx, order.URI); err != nil {
		logger.Errorf("WaitOrder(%q): %v", order.URI, err)
		return err
	}

	csr, certkey, err := newCSR(ids[0])
	if err != nil {
		logger.Errorf("newCSR(%q): %v", ids[0], err)
		return err
	}

	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		logger.Errorf("CreateOrderCert: %v", err)
		return err
	}

	leaf, err := validCert(der, certkey)
	if err != nil {
		logger.Errorf("invalid cert: %v", err)
		return err
	}
	cert := &tls.Certificate{
		PrivateKey:  certkey,
		Certificate: der,
		Leaf:        leaf,
	}

	d.certificate.Store(cert)
	acm.storage.putCert(d.nameInPunyCode, cert)
	return nil
}

func (d *Domain) renewCert(acm *AutoCertManager) {
	logger.Infof("begin renew certificate for domain %s", d.Name)

	for {
		err := d.doRenewCert(acm)
		if err == nil {
			break
		}
		logger.Errorf("failed to renew cerficate for domain %s: %v", d.Name, err)

		select {
		case <-acm.stopCtx.Done():
			return
		case <-time.After(time.Minute):
		}
	}

	d.setCertState(CertStateNormal)
	logger.Infof("certificate for domain %s has been renewed", d.Name)
}
