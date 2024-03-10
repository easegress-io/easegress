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

package autocertmanager

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/libdns/libdns"
	"github.com/megaease/easegress/v2/pkg/logger"
	"golang.org/x/crypto/acme"
)

// Domain represents a domain for automated certificate management
type Domain struct {
	*DomainSpec
	nameInPunyCode string
	certificate    atomic.Value
	cleanups       []func() error
	ctx            context.Context
}

// isWildcard returns whether the domain is for a wildcard one
func (d *Domain) isWildcard() bool {
	return d.nameInPunyCode[0] == '*'
}

// cert returns the certificate for the domain
func (d *Domain) cert() *tls.Certificate {
	if x := d.certificate.Load(); x != nil {
		return x.(*tls.Certificate)
	}
	return nil
}

func (d *Domain) certExpireTime() time.Time {
	cert := d.cert()
	if cert == nil {
		return time.Time{}
	}
	return cert.Leaf.NotAfter
}

func (d *Domain) updateCert(cert *tls.Certificate) {
	for {
		oldCert := d.cert()
		if oldCert == nil {
			if d.certificate.CompareAndSwap(nil, cert) {
				break
			}
			continue
		}
		if !cert.Leaf.NotAfter.After(oldCert.Leaf.NotAfter) {
			break
		}
		if d.certificate.CompareAndSwap(oldCert, cert) {
			break
		}
	}
}

func (d *Domain) runHTTP01(acm *AutoCertManager, chal *acme.Challenge) error {
	client := acm.client

	path := client.HTTP01ChallengePath(chal.Token)
	body, err := client.HTTP01ChallengeResponse(chal.Token)
	if err != nil {
		logger.Errorf("HTTP01ChallengeResponse: %v", err)
		return err
	}

	err = acm.storage.putHTTPToken(d.nameInPunyCode, path, []byte(body))
	if err != nil {
		logger.Errorf("put HTTP01 token: %v", err)
		return err
	}

	d.cleanups = append(d.cleanups, func() error {
		return acm.storage.deleteHTTPToken(d.nameInPunyCode, path)
	})

	_, err = client.Accept(d.ctx, chal)
	if err != nil {
		logger.Errorf("accept HTTP01 challenge: %v", err)
	}

	return err
}

func (d *Domain) runTLSALPN01(acm *AutoCertManager, z *acme.Authorization, chal *acme.Challenge) error {
	client := acm.client

	cert, err := client.TLSALPN01ChallengeCert(chal.Token, z.Identifier.Value)
	if err != nil {
		logger.Errorf("TLSALPN01ChallengeCert: %v", err)
		return err
	}

	err = acm.storage.putTLSALPNCert(d.nameInPunyCode, &cert)
	if err != nil {
		logger.Errorf("put TLSALPN01 certificate: %v", err)
		return err
	}

	d.cleanups = append(d.cleanups, func() error {
		acm.storage.deleteTLSALPNCert(d.nameInPunyCode)
		return nil
	})

	_, err = client.Accept(d.ctx, chal)
	if err != nil {
		logger.Errorf("accept TLSALPN01 challenge: %v", err)
	}
	return err
}

func (d *Domain) waitDNSRecord(value string) error {
	name := "_acme-challenge."
	if d.isWildcard() {
		name += d.nameInPunyCode[2:] // skip '*.'
	} else {
		name += d.nameInPunyCode
	}

	r := net.DefaultResolver
	if addr := d.DNSProvider["nsAddress"]; addr != "" {
		network := d.DNSProvider["nsNetwork"]
		r := net.Resolver{PreferGo: true}
		r.Dial = func(ctx context.Context, n, a string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			if network != "" {
				n = network
			}
			return d.DialContext(ctx, n, addr)
		}
	}

	for {
		select {
		case <-time.After(5 * time.Second):
		case <-d.ctx.Done():
			return d.ctx.Err()
		}

		if values, err := r.LookupTXT(d.ctx, name); err == nil {
			for _, v := range values {
				if v == value {
					return nil
				}
			}
		}
	}
}

func (d *Domain) deleteDuplicateRecord(dp dnsProvider, record libdns.Record) {
	if record.ID != "" {
		_, err := dp.DeleteRecords(d.ctx, d.Zone(), []libdns.Record{record})
		if err != nil {
			logger.Warnf("delete record id %s name %s failed, %v", record.ID, record.Name, err)
		}
		return
	}

	allRecords, err := dp.GetRecords(d.ctx, d.Zone())
	if err != nil {
		logger.Errorf("get records from zone %s failed, %v", d.Zone(), err)
		_, err := dp.DeleteRecords(d.ctx, d.Zone(), []libdns.Record{record})
		if err != nil {
			logger.Warnf("delete records %s from zone %s failed, %v", record.Name, d.Zone(), err)
		}
		return
	}

	diffNames := []string{}
	delRecords := []libdns.Record{}
	for _, r := range allRecords {
		if r.Type == record.Type && r.Name == record.Name {
			delRecords = append(delRecords, r)
		} else if r.Type == record.Type {
			diffNames = append(diffNames, r.Name)
		}
	}
	// for debug
	logger.Infof("dns records with same type %s but different names %v, append record %s", record.Type, diffNames, record.Name)
	if len(delRecords) > 0 {
		_, err := dp.DeleteRecords(d.ctx, d.Zone(), delRecords)
		if err != nil {
			logger.Warnf("delete records %v from zone %s failed, %v", delRecords, d.Zone(), err)
		}
		return
	}

	// del records is empty, try to delete record directly.
	_, err = dp.DeleteRecords(d.ctx, d.Zone(), []libdns.Record{record})
	if err != nil {
		logger.Warnf("delete records with name %s from zone %s failed, %v", record.Name, d.Zone(), err)
	}
}

func (d *Domain) runDNS01(acm *AutoCertManager, chal *acme.Challenge) error {
	client := acm.client

	value, err := client.DNS01ChallengeRecord(chal.Token)
	if err != nil {
		logger.Errorf("DNS01ChallengeRecord: %v", err)
		return err
	}

	dp, err := newDNSProvider(d.DomainSpec)
	if err != nil {
		logger.Errorf("new DNS provider: %v", err)
		return err
	}

	name := "_acme-challenge."
	if d.isWildcard() {
		name += d.nameInPunyCode[2:] // skip '*.'
	} else {
		name += d.nameInPunyCode
	}
	name = name[0 : len(name)-len(d.Zone())-1]
	record := libdns.Record{Type: "TXT", Name: name}

	d.deleteDuplicateRecord(dp, record)

	record.Value = value
	_, err = dp.AppendRecords(d.ctx, d.Zone(), []libdns.Record{record})
	if err != nil {
		logger.Errorf("AppendRecords: %v", err)
		return err
	}

	// we need to wait because it takes some time for a new DNS record to become effective
	err = d.waitDNSRecord(value)
	if err != nil {
		logger.Errorf("wait DNS record: %v", err)
		return err
	}

	_, err = client.Accept(d.ctx, chal)
	if err != nil {
		logger.Errorf("accept DNS01 challenge: %v", err)
	}
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

func (d *Domain) fulfill(acm *AutoCertManager, u string) error {
	client := acm.client

	z, err := client.GetAuthorization(d.ctx, u)
	if err != nil {
		logger.Errorf("GetAuthorization(%q): %v", u, err)
		return err
	}
	if z.Status != acme.StatusPending {
		return nil
	}
	d.cleanups = append(d.cleanups, func() error {
		// we cannot use d.ctx here as it may be already cancelled
		err := client.RevokeAuthorization(context.Background(), u)
		if err != nil {
			logger.Errorf("RevokeAuthorization(%q): %v", u, err)
		}
		return err
	})

	fulfilled := 0
	for _, chal := range z.Challenges {
		switch chal.Type {
		case "http-01":
			if acm.spec.EnableHTTP01 && d.runHTTP01(acm, chal) == nil {
				fulfilled++
			}
		case "dns-01":
			if acm.spec.EnableDNS01 && d.runDNS01(acm, chal) == nil {
				fulfilled++
			}
		case "tls-alpn-01":
			if acm.spec.EnableTLSALPN01 && d.runTLSALPN01(acm, z, chal) == nil {
				fulfilled++
			}
		default:
			logger.Errorf("unknown challenge type %q", chal.Type)
		}
	}
	if fulfilled == 0 {
		err = fmt.Errorf("no challenge was fulfilled")
		logger.Errorf("%v", err)
		return err
	}

	_, err = client.WaitAuthorization(d.ctx, z.URI)
	if err != nil {
		logger.Errorf("WaitAuthorization(%q): %v", z.URI, err)
	}

	return nil
}

func (d *Domain) renewCert(acm *AutoCertManager) error {
	ctx, cancel := context.WithTimeout(acm.stopCtx, 10*time.Minute)
	d.ctx = ctx

	defer func() {
		cleanups := d.cleanups
		d.cleanups = nil
		for _, fn := range cleanups {
			fn()
		}

		d.ctx = nil
		cancel()
	}()

	client := acm.client
	ids := acme.DomainIDs(d.nameInPunyCode)
	order, err := client.AuthorizeOrder(ctx, ids)
	if err != nil {
		logger.Errorf("AuthorizeOrder: %v", err)
		return err
	}

	for _, u := range order.AuthzURLs {
		if err = d.fulfill(acm, u); err != nil {
			return err
		}
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

	d.updateCert(cert)
	acm.storage.putCert(d.nameInPunyCode, cert)
	return nil
}
