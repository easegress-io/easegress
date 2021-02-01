package validator

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/megaease/easegateway/pkg/context"
)

// JWTValidatorSpec defines the configuration of JWT validator
type JWTValidatorSpec struct {
	Algorithm string `yaml:"algorithm" jsonschema:"enum=HS256,enum=HS384,enum=HS512"`
	// Secret is in hex encoding
	Secret string `yaml:"secret" jsonschema:"required,pattern=^[A-Fa-f0-9]+$"`
	// CookieName specifies the name of a cookie, if not empty, and the cookie with
	// this name both exists and has a non-empty value, its value is used as token
	// string, the Authorization header is used to get the token string otherwise.
	CookieName string `yaml:"cookieName" jsonschema:"omitempty"`
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(spec *JWTValidatorSpec) *JWTValidator {
	secret, _ := hex.DecodeString(spec.Secret)
	return &JWTValidator{
		spec:        spec,
		secretBytes: secret,
	}
}

// JWTValidator defines the JWT validator
type JWTValidator struct {
	spec        *JWTValidatorSpec
	secretBytes []byte
}

// Validate validates the JWT token of a http request
func (v *JWTValidator) Validate(req context.HTTPRequest) error {
	var token string

	if v.spec.CookieName != "" {
		if cookie, e := req.Cookie(v.spec.CookieName); e == nil {
			token = cookie.Value
		}
	}

	if token == "" {
		const prefix = "Bearer "
		authHdr := req.Header().Get("Authorization")
		if !strings.HasPrefix(authHdr, prefix) {
			return fmt.Errorf("unexpected authrization header: %s", authHdr)
		}
		token = authHdr[len(prefix):]
	}

	// jwt.Parse does everything incuding parsing and verification
	_, e := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if alg := token.Method.Alg(); alg != v.spec.Algorithm {
			return nil, fmt.Errorf("unexpected signing method: %v", alg)
		}
		return v.secretBytes, nil
	})

	return e
}
