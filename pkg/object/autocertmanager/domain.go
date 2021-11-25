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
	"sync/atomic"
	"time"

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
	if !acm.spec.EnableDNS01 {
		return nil
	}
	d.cleanup = append(d.cleanup, func() error {
		return nil
	})
	return nil
}

func newCSR(id acme.AuthzID) ([]byte, crypto.Signer) {
	var csr x509.CertificateRequest

	csr.DNSNames = append(csr.DNSNames, id.Value)
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("newCSR: ecdsa.GenerateKey for a cert: %v", err))
	}
	b, err := x509.CreateCertificateRequest(rand.Reader, &csr, k)
	if err != nil {
		panic(fmt.Sprintf("newCSR: x509.CreateCertificateRequest: %v", err))
	}
	return b, k
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
				//d.runDNS01(ctx, z, chal)
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
		logger.Errorf("WaitOrder: %v", err)
		return err
	}

	csr, certkey := newCSR(ids[0])
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
