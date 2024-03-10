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

package validator

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v4"

	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

// JWTValidatorSpec defines the configuration of JWT validator
type JWTValidatorSpec struct {
	Algorithm string `json:"algorithm" jsonschema:"enum=HS256,enum=HS384,enum=HS512,enum=RS256,enum=RS384,enum=RS512,enum=ES256,enum=ES384,enum=ES512,enum=EdDSA"`
	// PublicKey is in hex encoding
	PublicKey string `json:"publicKey" jsonschema:"pattern=^$|^[A-Fa-f0-9]+$"`
	// Secret is in hex encoding
	Secret string `json:"secret" jsonschema:"pattern=^$|^[A-Fa-f0-9]+$"`
	// CookieName specifies the name of a cookie, if not empty, and the cookie with
	// this name both exists and has a non-empty value, its value is used as token
	// string, the Authorization header is used to get the token string otherwise.
	CookieName string `json:"cookieName,omitempty"`
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(spec *JWTValidatorSpec) *JWTValidator {
	var key interface{}
	if len(spec.PublicKey) > 0 {
		publicKeyBytes, _ := hex.DecodeString(spec.PublicKey)
		p, _ := pem.Decode(publicKeyBytes)
		key, _ = x509.ParsePKIXPublicKey(p.Bytes)
	} else {
		key, _ = hex.DecodeString(spec.Secret)
	}
	return &JWTValidator{
		spec: spec,
		key:  key,
	}
}

// JWTValidator defines the JWT validator
type JWTValidator struct {
	spec *JWTValidatorSpec
	key  interface{}
}

// Validate validates the JWT token of a http request
func (v *JWTValidator) Validate(req *httpprot.Request) error {
	var token string

	if v.spec.CookieName != "" {
		if cookie, e := req.Cookie(v.spec.CookieName); e == nil {
			token = cookie.Value
		}
	}

	if token == "" {
		const prefix = "Bearer "
		authHdr := req.HTTPHeader().Get("Authorization")
		if !strings.HasPrefix(authHdr, prefix) {
			return fmt.Errorf("unexpected authorization header: %s", authHdr)
		}
		token = authHdr[len(prefix):]
	}
	// jwt.Parse does everything including parsing and verification
	t, e := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if alg := token.Method.Alg(); alg != v.spec.Algorithm {
			return nil, fmt.Errorf("unexpected signing method: %v", alg)
		}
		return v.key, nil
	})
	if e != nil {
		return e
	}
	if !t.Valid {
		return fmt.Errorf("invalid jwt token")
	}
	return nil
}
