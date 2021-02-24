package httpserver

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/megaease/easegateway/pkg/tracing"
	"github.com/megaease/easegateway/pkg/util/ipfilter"
)

type (
	// Spec describes the HTTPServer.
	Spec struct {
		HTTP3            bool          `yaml:"http3" jsonschema:"omitempty"`
		Port             uint16        `yaml:"port" jsonschema:"required,minimum=1"`
		KeepAlive        bool          `yaml:"keepAlive" jsonschema:"required"`
		KeepAliveTimeout string        `yaml:"keepAliveTimeout" jsonschema:"omitempty,format=duration"`
		MaxConnections   uint32        `yaml:"maxConnections" jsonschema:"omitempty,minimum=1"`
		HTTPS            bool          `yaml:"https" jsonschema:"required"`
		CertBase64       string        `yaml:"certBase64" jsonschema:"omitempty,format=base64"`
		KeyBase64        string        `yaml:"keyBase64" jsonschema:"omitempty,format=base64"`
		CacheSize        uint32        `yaml:"cacheSize" jsonschema:"omitempty"`
		XForwardedFor    bool          `yaml:"xForwardedFor" jsonschema:"omitempty"`
		Tracing          *tracing.Spec `yaml:"tracing" jsonschema:"omitempty"`

		IPFilter *ipfilter.Spec `yaml:"ipFilter,omitempty" jsonschema:"omitempty"`
		Rules    []Rule         `yaml:"rules" jsonschema:"omitempty"`
	}

	// Rule is first level entry of router.
	Rule struct {
		// NOTICE: If the field is a pointer, it must have `omitempty` in tag `yaml`
		// when it has `omitempty` in tag `jsonschema`.
		// Otherwise it will output null value, which is invalid in json schema (the type is object).
		// the original reason is the jsonscheme(genjs) has not support multiple types.
		// Reference: https://github.com/alecthomas/jsonschema/issues/30
		// In the future if we have the scienrio where we need marshal the field, but omitempty
		// in the schema, we are suppose to support multuple types on our own.
		IPFilter   *ipfilter.Spec `yaml:"ipFilter,omitempty" jsonschema:"omitempty"`
		Host       string         `yaml:"host" jsonschema:"omitempty"`
		HostRegexp string         `yaml:"hostRegexp" jsonschema:"omitempty,format=regexp"`
		Paths      []Path         `yaml:"paths" jsonschema:"omitempty"`

		hostRE *regexp.Regexp
	}

	// Path is second level entry of router.
	Path struct {
		IPFilter   *ipfilter.Spec `yaml:"ipFilter,omitempty" jsonschema:"omitempty"`
		Path       string         `yaml:"path,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathPrefix string         `yaml:"pathPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathRegexp string         `yaml:"pathRegexp,omitempty" jsonschema:"omitempty,format=regexp"`
		Methods    []string       `yaml:"methods,omitempty" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
		Backend    string         `yaml:"backend" jsonschema:"required"`
		Headers    []*Header      `yaml:"headers" jsonschema:"omitempty"`

		pathRE *regexp.Regexp
	}

	// Header is the third level entry of router. A header entry is always under a specific path entry, that is to mean
	// the headers entry will only be checked after a path entry matched. However, the headers entry has a higher priority
	// than the path entry itself.
	Header struct {
		Key     string   `yaml:"key" jsonschema:"required"`
		Values  []string `yaml:"values,omitempty" jsonschema:"omitempty,uniqueItems=true"`
		Regexp  string   `yaml:"regexp,omitempty" jsonschema:"omitempty,format=regexp"`
		Backend string   `yaml:"backend" jsonschema:"required"`

		headerRE *regexp.Regexp
	}
)

// Validate validates HTTPServerSpec.
func (spec *Spec) Validate() error {
	if spec.HTTP3 && !spec.HTTPS {
		return fmt.Errorf("https is disabled when http3 enabled")
	}

	if spec.HTTPS {
		if spec.CertBase64 == "" {
			return fmt.Errorf("certBase64 is empty when https enabled")
		}
		if spec.KeyBase64 == "" {
			return fmt.Errorf("keyBase64 is empty when https enabled")
		}
		_, err := spec.tlsConfig()
		if err != nil {
			return err
		}
	}

	return nil
}

func (spec Spec) tlsConfig() (*tls.Config, error) {
	certPem, _ := base64.StdEncoding.DecodeString(spec.CertBase64)
	keyPem, _ := base64.StdEncoding.DecodeString(spec.KeyBase64)

	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return nil, fmt.Errorf("generate x509 key pair failed: %v", err)
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

func (h *Header) initHeaderRoute() {
	h.headerRE = regexp.MustCompile(h.Regexp)
}

// Validate validates Header.
func (h *Header) Validate() error {
	if len(h.Values) == 0 && h.Regexp == "" {
		return fmt.Errorf("both of values and regexp are empty for key: %s", h.Key)
	}

	return nil
}
