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
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/logger"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

const (
	autoCertManagerCertPrefix  = "autocert/cert/"
	autoCertManagerCert        = "autocert/cert/%s"
	autoCertManagerHTTPToken   = "autocert/http/%s/%s"
	autoCertManagerTLSALPNCert = "autocert/tlsalpn/%s"
)

type storage struct {
	cls cluster.Cluster
}

func domainCert(name string) string {
	return fmt.Sprintf(autoCertManagerCert, name)
}

func domainHTTPToken(name, path string) string {
	return fmt.Sprintf(autoCertManagerHTTPToken, name, path)
}

func domainTLSALPNCert(name string) string {
	return fmt.Sprintf(autoCertManagerTLSALPNCert, name)
}

func encodeCertificate(cert *tls.Certificate) ([]byte, error) {
	// contains PEM-encoded data
	var buf bytes.Buffer

	// private
	switch key := cert.PrivateKey.(type) {
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, err
		}
		pb := &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
		if err := pem.Encode(&buf, pb); err != nil {
			return nil, err
		}
	case *rsa.PrivateKey:
		b := x509.MarshalPKCS1PrivateKey(key)
		pb := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: b}
		if err := pem.Encode(&buf, pb); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown private key type")
	}

	// public
	for _, b := range cert.Certificate {
		pb := &pem.Block{Type: "CERTIFICATE", Bytes: b}
		if err := pem.Encode(&buf, pb); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// Attempt to parse the given private key DER block. OpenSSL 0.9.8 generates
// PKCS#1 private keys by default, while OpenSSL 1.0.0 generates PKCS#8 keys.
// OpenSSL ecparam generates SEC1 EC private keys for ECDSA. We try all three.
//
// Inspired by parsePrivateKey in crypto/tls/tls.go.
func parsePrivateKey(der []byte) (crypto.Signer, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey:
			return key, nil
		case *ecdsa.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}

	return nil, fmt.Errorf("failed to parse private key")
}

// validCert parses a cert chain provided as der argument and verifies the leaf and der[0]
// correspond to the private key, the domain and key type match, and expiration dates
// are valid. It doesn't do any revocation checking.
//
// The returned value is the verified leaf cert.
func validCert(der [][]byte, key crypto.Signer) (leaf *x509.Certificate, err error) {
	now := time.Now()

	// parse public part(s)
	var n int
	for _, b := range der {
		n += len(b)
	}
	pub := make([]byte, n)
	n = 0
	for _, b := range der {
		n += copy(pub[n:], b)
	}
	x509Cert, err := x509.ParseCertificates(pub)
	if err != nil || len(x509Cert) == 0 {
		return nil, fmt.Errorf("no public key found")
	}
	// verify the leaf is not expired and matches the domain name
	leaf = x509Cert[0]
	if now.Before(leaf.NotBefore) {
		return nil, fmt.Errorf("certificate is not valid yet")
	}
	if now.After(leaf.NotAfter) {
		return nil, fmt.Errorf("expired certificate")
	}
	// ensure the leaf corresponds to the private key and matches the certKey type
	switch pub := leaf.PublicKey.(type) {
	case *rsa.PublicKey:
		prv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key type does not match public key type")
		}
		if pub.N.Cmp(prv.N) != 0 {
			return nil, fmt.Errorf("private key does not match public key")
		}
	case *ecdsa.PublicKey:
		prv, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key type does not match public key type")
		}
		if pub.X.Cmp(prv.X) != 0 || pub.Y.Cmp(prv.Y) != 0 {
			return nil, fmt.Errorf("private key does not match public key")
		}
	default:
		return nil, fmt.Errorf("unknown public key algorithm")
	}
	return leaf, nil
}

func decodeCertificate(data []byte) (*tls.Certificate, error) {
	// private
	priv, pub := pem.Decode(data)
	if priv == nil || !strings.Contains(priv.Type, "PRIVATE") {
		return nil, fmt.Errorf("no private key found")
	}
	privKey, err := parsePrivateKey(priv.Bytes)
	if err != nil {
		return nil, err
	}

	// public
	var pubDER [][]byte
	for len(pub) > 0 {
		var b *pem.Block
		b, pub = pem.Decode(pub)
		if b == nil {
			break
		}
		pubDER = append(pubDER, b.Bytes)
	}
	if len(pub) > 0 {
		// Leftover content not consumed by pem.Decode. Corrupt. Ignore.
		return nil, fmt.Errorf("public key corrupt")
	}

	// verify and create TLS cert
	leaf, err := validCert(pubDER, privKey)
	if err != nil {
		return nil, err
	}
	tlscert := &tls.Certificate{
		Certificate: pubDER,
		PrivateKey:  privKey,
		Leaf:        leaf,
	}
	return tlscert, nil
}

// newStorage creates a storage.
func newStorage(cls cluster.Cluster) *storage {
	return &storage{cls: cls}
}

func (s *storage) getCert(domain string) (*tls.Certificate, error) {
	key := domainCert(domain)

	kv, err := s.cls.GetRaw(key)
	if err != nil {
		return nil, err
	}
	if kv == nil {
		return nil, fmt.Errorf("certificate does not exist")
	}

	return decodeCertificate(kv.Value)
}

func (s *storage) putCert(domain string, cert *tls.Certificate) error {
	key := domainCert(domain)
	value, err := encodeCertificate(cert)
	if err != nil {
		return err
	}
	return s.cls.Put(key, string(value))
}

func (s *storage) getHTTPToken(domain, path string) ([]byte, error) {
	key := domainHTTPToken(domain, path)
	kv, err := s.cls.GetRaw(key)
	if err != nil {
		return nil, err
	}
	if kv == nil {
		return nil, fmt.Errorf("token does not exist")
	}
	return kv.Value, nil
}

func (s *storage) putHTTPToken(domain, path string, value []byte) error {
	key := domainHTTPToken(domain, path)
	return s.cls.Put(key, string(value))
}

func (s *storage) deleteHTTPToken(domain, path string) error {
	key := domainHTTPToken(domain, path)
	return s.cls.Delete(key)
}

func (s *storage) getTLSALPNCert(domain string) (*tls.Certificate, error) {
	key := domainTLSALPNCert(domain)
	kv, err := s.cls.GetRaw(key)
	if err != nil {
		return nil, err
	}
	if kv == nil {
		return nil, fmt.Errorf("tls-alpn certificate does not exist")
	}
	return decodeCertificate(kv.Value)
}

func (s *storage) putTLSALPNCert(domain string, cert *tls.Certificate) error {
	key := domainTLSALPNCert(domain)
	value, err := encodeCertificate(cert)
	if err != nil {
		return err
	}
	return s.cls.Put(key, string(value))
}

func (s *storage) deleteTLSALPNCert(domain string) error {
	key := domainTLSALPNCert(domain)
	return s.cls.Delete(key)
}

func (s *storage) watchCertificate(ctx context.Context, onChange func(domain string, cert *tls.Certificate)) {
	var (
		syncer cluster.Syncer
		err    error
		ch     <-chan map[string]*mvccpb.KeyValue
	)

	for {
		syncer, err = s.cls.Syncer(time.Hour)
		if err != nil {
			logger.Errorf("failed to create syncer: %v", err)
		} else if ch, err = syncer.SyncRawPrefix(autoCertManagerCertPrefix); err != nil {
			logger.Errorf("failed to sync raw prefix: %v", err)
			syncer.Close()
		} else {
			break
		}

		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			return
		}
	}
	defer syncer.Close()

	fn := func(kvs map[string]*mvccpb.KeyValue) {
		for _, kv := range kvs {
			domain := strings.TrimPrefix(string(kv.Key), autoCertManagerCertPrefix)
			cert, err := decodeCertificate(kv.Value)
			if err == nil {
				onChange(domain, cert)
			} else {
				logger.Errorf("failed to decode certificate for %q: %v", domain, err)
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case kvs := <-ch:
			fn(kvs)
		}
	}
}
