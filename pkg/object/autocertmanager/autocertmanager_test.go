package autocertmanager

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	cluster "github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
	"github.com/megaease/easegress/pkg/supervisor"
	"golang.org/x/crypto/acme"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestSpecValidate(t *testing.T) {
	t.Run("no enabled challenge", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
enableDNS01: false
enableHTTP01: false
enableTLSALPN01: false
domains:
  - name: "*.megaease.com"
`
		_, err := supervisor.NewSpec(yaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})

	t.Run("invalid characters", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
enableDNS01: false
domains:
  - name: "*.meg@ease.com"
    dnsProvider:
      name: alidns
      zone: megaease.com
`
		_, err := supervisor.NewSpec(yaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})

	t.Run("wildcard without DNS01", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
enableDNS01: false
domains:
  - name: "*.megaease.com"
    dnsProvider:
      name: alidns
      zone: megaease.com
`
		_, err := supervisor.NewSpec(yaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})

	t.Run("unknow DNS provider", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
domains:
  - name: "*.megaease.com"
    dnsProvider:
      name: unknown
      zone: megaease.com
`
		_, err := supervisor.NewSpec(yaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})

	t.Run("normal", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
domains:
  - name: "www.megaease.com"
`
		_, err := supervisor.NewSpec(yaml)
		if err != nil {
			t.Errorf("spec creation should have succeeded: %v", err)
		}
	})
}

func TestDNSProvider(t *testing.T) {
	spec := &DomainSpec{
		Name: "www.megaease.com",
	}
	t.Run("no DNS provider", func(t *testing.T) {
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("no DNS provider name", func(t *testing.T) {
		spec.DNSProvider = map[string]string{}
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("no zone", func(t *testing.T) {
		spec.DNSProvider["name"] = "alidns"
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("unknown DNS provider", func(t *testing.T) {
		spec.DNSProvider["name"] = "unknown"
		spec.DNSProvider["zone"] = "megaease.com"
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("lack required fields", func(t *testing.T) {
		spec.DNSProvider["name"] = "alidns"
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("alidns", func(t *testing.T) {
		spec.DNSProvider["name"] = "alidns"
		spec.DNSProvider["accessKeyId"] = "accessKeyId"
		spec.DNSProvider["accessKeySecret"] = "accessKeySecret"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("azure", func(t *testing.T) {
		spec.DNSProvider["name"] = "azure"
		spec.DNSProvider["tenantId"] = "tenantId"
		spec.DNSProvider["clientId"] = "clientId"
		spec.DNSProvider["clientSecret"] = "clientSecret"
		spec.DNSProvider["subscriptionId"] = "subscriptionId"
		spec.DNSProvider["resourceGroupName"] = "resourceGroupName"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("cloudflare", func(t *testing.T) {
		spec.DNSProvider["name"] = "cloudflare"
		spec.DNSProvider["apiToken"] = "apiToken"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("digitalocean", func(t *testing.T) {
		spec.DNSProvider["name"] = "digitalocean"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("dnspod", func(t *testing.T) {
		spec.DNSProvider["name"] = "dnspod"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("duckdns", func(t *testing.T) {
		spec.DNSProvider["name"] = "duckdns"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("google", func(t *testing.T) {
		spec.DNSProvider["name"] = "google"
		spec.DNSProvider["project"] = "project"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("hetzner", func(t *testing.T) {
		spec.DNSProvider["name"] = "hetzner"
		spec.DNSProvider["authApiToken"] = "authApiToken"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("route53", func(t *testing.T) {
		spec.DNSProvider["name"] = "route53"
		spec.DNSProvider["accessKeyId"] = "accessKeyId"
		spec.DNSProvider["secretAccessKey"] = "secretAccessKey"
		spec.DNSProvider["awsProfile"] = "awsProfile"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("vultr", func(t *testing.T) {
		spec.DNSProvider["name"] = "vultr"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})
}

// https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L44
var discoTmpl = template.Must(template.New("disco").Parse(`{
	"new-reg": "{{.}}/new-reg",
	"newAccount": "{{.}}/new-reg",
	"new-authz": "{{.}}/new-authz",
	"newAuthz": "{{.}}/new-authz",
	"new-cert": "{{.}}/new-cert",
	"newOrder": "{{.}}/new-cert"
}`))

// //Reg          string `json:"new-reg"`
// //RegRFC       string `json:"newAccount"`
// Authz        string `json:"new-authz"`
// //AuthzRFC     string `json:"newAuthz"`
// OrderRFC     string `json:"newOrder"`
// //Cert         string `json:"new-cert"`
// Revoke       string `json:"revoke-cert"`
// RevokeRFC    string `json:"revokeCert"`
// NonceRFC     string `json:"newNonce"`
// KeyChangeRFC string `json:"keyChange"`
// Meta         struct {
// 	Terms           string   `json:"terms-of-service"`
// 	TermsRFC        string   `json:"termsOfService"`
// 	WebsiteRFC      string   `json:"website"`
// 	CAA             []string `json:"caa-identities"`
// 	CAARFC          []string `json:"caaIdentities"`
// 	ExternalAcctRFC bool     `json:"externalAccountRequired"`

// https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L50
var authzTmpl = template.Must(template.New("authz").Parse(`{
	"status": "pending",
	"challenges": [
		{
			"uri": "{{.}}/challenge/tls-alpn-01",
			"type": "tls-alpn-01",
			"token": "token-alpn"
		},
		{
			"uri": "{{.}}/challenge/dns-01",
			"type": "dns-01",
			"token": "token-dns-01"
		},
		{
			"uri": "{{.}}/challenge/http-01",
			"type": "http-01",
			"token": "token-http-01"
		}
	]
}`))

// https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L170
func decodePayload(v interface{}, r io.Reader) error {
	var req struct{ Payload string }
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return err
	}
	payload, err := base64.RawURLEncoding.DecodeString(req.Payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, v)
}

// Copied from https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L146
func dummyCert(pub interface{}, san ...string) ([]byte, error) {
	return dateDummyCert(pub, time.Now(), time.Now().Add(90*24*time.Hour), san...)
}

func dateDummyCert(pub interface{}, start, end time.Time, san ...string) ([]byte, error) {
	// use EC key to run faster on 386
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	t := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             start,
		NotAfter:              end,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageKeyEncipherment,
		DNSNames:              san,
	}
	if pub == nil {
		pub = &key.PublicKey
	}
	return x509.CreateCertificate(rand.Reader, t, t, pub, key)
}

// Inspired by https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L477
func startACMEServerStub(t *testing.T, domain string, wg *sync.WaitGroup) (url string, finish func()) {
	// ACME CA server stub
	csrContainer := make([]byte, 0)
	var ca *httptest.Server
	ca = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", "nonce")
		// Directory request.
		if r.Method == "HEAD" {
			// a nonce request
			return
		}

		switch r.URL.Path {
		// discovery
		case "/":
			if err := discoTmpl.Execute(w, ca.URL); err != nil {
				t.Errorf("discoTmpl: %v", err)
			}
		// client key registration
		case "/new-reg":
			w.Header().Set("Location", ca.URL+"/new-reg/1")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("{}"))
		// domain authorization
		case "/new-authz":
			w.Header().Set("Location", ca.URL+"/authz/1")
			w.WriteHeader(http.StatusCreated)
			if err := authzTmpl.Execute(w, ca.URL); err != nil {
				t.Errorf("authzTmpl: %v", err)
			}
		// cert request
		case "/new-cert":
			w.Header().Set("Location", ca.URL+"/order/1")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(fmt.Sprintf(`{"status": "valid", "finalize": "%s/finalize/1"}`, ca.URL)))
		// CA chain cert
		case "/order/1":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "valid"}`))
		case "/finalize/1":
			var req struct {
				CSR string `json:"csr"`
			}
			err := decodePayload(&req, r.Body)
			if err != nil {
				t.Errorf("base64 decoding failed: %v", err)
			}
			b, err := base64.RawURLEncoding.DecodeString(req.CSR)
			if err != nil {
				t.Errorf("base64 decoding failed: %v", err)
			}
			csr, err := x509.ParseCertificateRequest(b)
			if err != nil {
				t.Errorf("new-cert: CSR: %v", err)
			}
			der, err := dummyCert(csr.PublicKey, domain)
			if err != nil {
				t.Errorf("ca-cert: dummyCert: %v", err)
			}
			csrContainer = der
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{"status": "valid", "certificate": "%s/cert/1"}`, ca.URL)))
		case "/cert/1":
			w.Header().Set("Content-Type", "application/pem-certificate-chain")
			pem.Encode(w, &pem.Block{Type: "CERTIFICATE", Bytes: csrContainer})
			wg.Done()
		default:
			t.Errorf("unrecognized r.URL.Path: %s", r.URL.Path)
		}
	}))
	finish = func() {
		ca.Close()
	}
	return ca.URL, finish
}

func helloInfo(sni string) *tls.ClientHelloInfo {
	return &tls.ClientHelloInfo{
		ServerName:   sni,
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
	}
}

func TestAutoCertManagerCreation(t *testing.T) {
	//TODO only to debug test
	logger.EtcdClientLoggerConfig(&option.Options{}, "blaa")

	acmWg := &sync.WaitGroup{}
	acmWg.Add(1)

	url, finish := startACMEServerStub(t, "www.example.com", acmWg)
	defer finish()

	yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
domains:
  - name: "www.megaease.com"
directoryURL: ` + url
	etcdDirName, err := ioutil.TempDir("", "autocertmanager-test")
	if err != nil {
		t.Errorf(err.Error())
	}
	defer os.RemoveAll(etcdDirName)

	cls := cluster.CreateClusterForTest(etcdDirName)
	//cls := &clustertest.MockedCluster{}
	supervisor.MustNew(&option.Options{}, cls)

	spec, err := supervisor.NewSpec(yaml)
	if err != nil {
		t.Errorf("spec creation should have succeeded: %v", err)
	}

	acm := &AutoCertManager{}
	acm.Init(spec)
	acmWg.Wait()
	time.Sleep(1 * time.Second)

	hello := helloInfo("example.org")
	hello.SupportedProtos = []string{acme.ALPNProto}
	GetCertificate(hello, false)

	hello.SupportedProtos = []string{"random protos"}
	GetCertificate(hello, false)
	GetCertificate(hello, true)

	// w http.ResponseWriter, r *http.Request
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleHTTP01Challenge(w, r)
		wg.Done()
	}))
	defer ts.Close()
	_, err = http.Get(ts.URL)
	if err != nil {
		t.Errorf(err.Error())
	}
	wg.Wait()

	if len(acm.Status().ObjectStatus.(*Status).Domains) != 1 {
		t.Error("bad status")
	}
	if acm.spec.Domains[0].Zone() != "" {
		t.Error("bad status")
	}
	acm.Inherit(spec, acm)
	acm.Close()

	closeWG := &sync.WaitGroup{}
	closeWG.Add(1)
	cls.CloseServer(closeWG)
	closeWG.Wait()
}

func TestAutoCertManagerCreationNotRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleHTTP01Challenge(w, r)
		wg.Done()
	}))
	defer ts.Close()
	_, err := http.Get(ts.URL)
	if err != nil {
		t.Errorf(err.Error())
	}
	wg.Wait()

	GetCertificate(helloInfo("example.org"), false)
}
