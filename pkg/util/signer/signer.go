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

package signer

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	dateFormat      = "20060102"
	timeFormat      = "20060102T150405Z"
	unsignedPayload = "UNSIGNED-PAYLOAD"
	sha256Empty     = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	authHeader      = "Authorization"
	hostHeader      = "host"
)

type (
	// Literal is the header name, query name and other text values.
	// The literals are string constants, but customers may want to
	// customize them to be their own, so we make them configurable.
	Literal struct {
		// ScopeSuffix is the last part when build the credential scope.
		// Default: megaease_request
		ScopeSuffix string `yaml:"scopeSuffix" json:"scopeSuffix" jsonschema:"required"`

		// AlgorithmName is the query name of the signature algorithm
		// Default: X-Me-Algorithm
		AlgorithmName string `yaml:"algorithmName" json:"algorithmName" jsonschema:"required"`

		// AlgorithmName is the header/query value of the signature algorithm
		// Default: ME-HMAC-SHA256
		AlgorithmValue string `yaml:"algorithmValue" json:"alrithmValue" jsonschema:"required"`

		// SignedHeaders is the header/query headers of the signed headers
		// Default: X-Me-SignedHeaders
		SignedHeaders string `yaml:"signedHeaders" json:"signedHeaders" jsonschema:"required"`

		// Signature is the query name of the signature
		// Default: X-Me-Signature
		Signature string `yaml:"signature" json:"signature" jsonschema:"required"`

		// Date is the header/query name of request time
		// Default: X-Me-Date
		Date string `yaml:"date" json:"date" jsonschema:"required"`

		// Expires is the query name of expire duration
		// Default: X-Me-Expires
		Expires string `yaml:"expires" json:"expires" jsonschema:"required"`

		// Credential is the query name of credential
		// Default: X-Me-Credential
		Credential string `yaml:"credential" json:"credential" jsonschema:"required"`

		// ContentSHA256 is the header name of body/payload hash
		// Default: X-Me-Content-Sha256
		ContentSHA256 string `yaml:"contentSha256" json:"contentSha256" jsonschema:"required"`

		// SigningKeyPrefix is prepend to access key secret when derive the signing key
		// Default: ME
		SigningKeyPrefix string `yaml:"signingKeyPrefix" json:"signingKeyPrefix" jsonschema:"omitempty"`
	}

	// HeaderHoisting defines which headers are allowed to be moved from header to query
	// in presign: header with name has one of the allowed prefixes, but hasn't any
	// disallowed prefixes and doesn't match any of disallowed names are allowed to be
	// hoisted
	HeaderHoisting struct {
		AllowedPrefix    []string `yaml:"allowedPrefix" json:"allowedPrefix" jsonschema:"omitempty,uniqueItems=true"`
		DisallowedPrefix []string `yaml:"disallowedPrefix" json:"disallowedPrefix" jsonschema:"omitempty,uniqueItems=true"`
		Disallowed       []string `yaml:"disallowed" json:"disallowed" jsonschema:"omitempty,uniqueItems=true"`
		disallowed       map[string]bool
	}

	// AccessKeyStore defines the interface of an access key store, which returns the
	// corresponding secret when query by an id
	AccessKeyStore interface {
		GetSecret(id string) (string, bool)
	}

	// Signer is a signature calculator for http.Request
	Signer struct {
		literal        *Literal
		ignoredHeaders map[string]bool
		headerHoisting *HeaderHoisting
		ttl            time.Duration
		excludeBody    bool
		// accessKeyID & accessKeySecret are for signing
		accessKeyID     string
		accessKeySecret string
		// accessKeyStore is for signature verification
		accessKeyStore AccessKeyStore
	}

	// SigningContext is the signing context for a single request
	SigningContext struct {
		*Signer
		isPresign bool

		Time        time.Time
		Scopes      []string
		scopeString string
		ExpireTime  time.Duration

		AccessKeyID      string
		AccessKeySecret  string
		SignedHeaders    string
		CanonicalHeaders string

		Signature string
		Query     url.Values
		BodyHash  string
	}
)

func formatDate(t time.Time) string {
	return t.Format(dateFormat)
}

func formatTime(t time.Time) string {
	return t.Format(timeFormat)
}

func hmacDigest(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

func sha256DegistAndEncodeToHexString(data []byte) string {
	hash := sha256.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

var noEscapeChars [256]bool

func init() {
	for i := range noEscapeChars {
		noEscapeChars[i] = (i >= 'A' && i <= 'Z') ||
			(i >= 'a' && i <= 'z') ||
			(i >= '0' && i <= '9') ||
			i == '-' ||
			i == '.' ||
			i == '_' ||
			i == '~'
	}
}

func buildCanonicalURI(u *url.URL) string {
	const hex = "0123456789ABCDEF"

	var uri string

	if len(u.Opaque) > 0 {
		uri = "/" + strings.Join(strings.Split(u.Opaque, "/")[3:], "/")
	} else {
		uri = u.EscapedPath()
	}

	if len(uri) == 0 {
		return "/"
	}

	var buf bytes.Buffer
	for i := 0; i < len(uri); i++ {
		c := uri[i]
		if noEscapeChars[c] || c == '/' {
			buf.WriteByte(c)
		} else {
			buf.WriteByte('%')
			buf.WriteByte(hex[c>>4])
			buf.WriteByte(hex[c&0x0f])
		}
	}
	return buf.String()
}

// SetLiteral is an option function for Signer to set literals
func (signer *Signer) SetLiteral(literal *Literal) *Signer {
	signer.literal = literal
	return signer
}

// ExcludeBody is an option function for Signer to exclude body from signature
func (signer *Signer) ExcludeBody(exclude bool) *Signer {
	signer.excludeBody = exclude
	return signer
}

// IgnoreHeader is an option function for Signer to add ignored headers
func (signer *Signer) IgnoreHeader(headers ...string) *Signer {
	for _, h := range headers {
		signer.ignoredHeaders[h] = true
	}
	return signer
}

// SetHeaderHoisting is an option function for Singer to set header hoisting
func (signer *Signer) SetHeaderHoisting(hh *HeaderHoisting) *Signer {
	hh.disallowed = map[string]bool{}
	for _, s := range hh.Disallowed {
		hh.disallowed[s] = true
	}
	signer.headerHoisting = hh
	return signer
}

// SetTTL is an option function for Signer to set time to live of a signature
func (signer *Signer) SetTTL(d time.Duration) *Signer {
	signer.ttl = d
	return signer
}

// SetAccessKeyStore is an option function for Signer to set access key store
func (signer *Signer) SetAccessKeyStore(store AccessKeyStore) *Signer {
	signer.accessKeyStore = store
	return signer
}

// SetCredential is an option function for Signer to set access key id/secret for signing
func (signer *Signer) SetCredential(accessKeyID string, accessKeySecret string) *Signer {
	signer.accessKeyID = accessKeyID
	signer.accessKeySecret = accessKeySecret
	return signer
}

var defaultLiteral = &Literal{
	ScopeSuffix:      "megaease_request",
	AlgorithmName:    "X-Me-Algorithm",
	AlgorithmValue:   "ME-HMAC-SHA256",
	SignedHeaders:    "X-Me-SignedHeaders",
	Signature:        "X-Me-Signature",
	Date:             "X-Me-Date",
	Expires:          "X-Me-Expires",
	Credential:       "X-Me-Credential",
	ContentSHA256:    "X-Me-Content-Sha256",
	SigningKeyPrefix: "ME",
}

// New creates a new signer
func New() *Signer {
	signer := &Signer{
		literal: defaultLiteral,
		ignoredHeaders: map[string]bool{
			authHeader:   true,
			"User-Agent": true,
		},
	}
	return signer
}

// NewContext creates a new signing context for signing
func (signer *Signer) NewContext(timestamp time.Time, scopes ...string) *SigningContext {
	ctx := &SigningContext{
		Signer:          signer,
		Time:            timestamp.UTC(),
		Scopes:          scopes,
		AccessKeyID:     signer.accessKeyID,
		AccessKeySecret: signer.accessKeySecret,
	}
	return ctx
}

func (ctx *SigningContext) buildScopeString() {
	if ctx.Time.IsZero() {
		ctx.Time = time.Now().UTC()
	}

	buf := bytes.Buffer{}
	buf.WriteString(formatDate(ctx.Time))
	for _, s := range ctx.Scopes {
		buf.WriteByte('/')
		buf.WriteString(s)
	}
	buf.WriteByte('/')
	buf.WriteString(ctx.literal.ScopeSuffix)
	ctx.scopeString = buf.String()
}

func (ctx *SigningContext) deriveSigningKey() []byte {
	key := []byte(ctx.literal.SigningKeyPrefix + ctx.AccessKeySecret)

	t := formatDate(ctx.Time)
	key = hmacDigest(key, []byte(t))
	for _, s := range ctx.Scopes {
		key = hmacDigest(key, []byte(s))
	}

	return hmacDigest(key, []byte(ctx.literal.ScopeSuffix))
}

func (ctx *SigningContext) getCanonicalQuery(u *url.URL) string {
	ctx.Query.Del(ctx.literal.Signature)

	if ctx.isPresign {
		ctx.Query.Set(ctx.literal.AlgorithmName, ctx.literal.AlgorithmValue)
		ctx.Query.Set(ctx.literal.Date, formatTime(ctx.Time))
		ctx.Query.Set(ctx.literal.Credential, ctx.AccessKeyID+"/"+ctx.scopeString)

		duration := int64(ctx.ExpireTime / time.Second)
		ctx.Query.Set(ctx.literal.Expires, strconv.FormatInt(duration, 10))

		ctx.Query.Set(ctx.literal.SignedHeaders, ctx.SignedHeaders)
	} else {
		ctx.Query.Del(ctx.literal.AlgorithmName)
		ctx.Query.Del(ctx.literal.Credential)
		ctx.Query.Del(ctx.literal.Date)
		ctx.Query.Del(ctx.literal.Expires)
		ctx.Query.Del(ctx.literal.SignedHeaders)
	}

	for _, v := range ctx.Query {
		sort.Strings(v)
	}
	return ctx.Query.Encode()
}

// for each str in strs
//   trim leading & trailing spaces
//   convert sequential spaces to single space
// then join all the strs with comma
func buildCanonicalHeaderValue(strs []string) string {
	var buf bytes.Buffer

	for i, str := range strs {
		var s, e int

		if i > 0 {
			buf.WriteByte(',')
		}

		// trim leading spaces
		for s = 0; s < len(str) && str[s] == ' '; s++ {
		}

		// trim trailing spaces
		for e = len(str); e > s && str[e-1] == ' '; e-- {
		}

		// convert sequential spaces to single space
		for m, spaces := s, 0; m < e; m++ {
			if str[m] == ' ' {
				spaces++
				continue
			}
			if spaces > 1 {
				buf.WriteString(str[s : m-spaces+1])
				s = m
			}
			spaces = 0
		}

		if s < e {
			buf.WriteString(str[s:e])
		}
	}

	return buf.String()
}

// get host of the request, port is removed if it is the default number,
// this function haven't handle all error cases
func getHost(req *http.Request) string {
	host := req.Host
	if host == "" {
		host = req.URL.Host
		if host == "" {
			return ""
		}
	}

	colon := strings.LastIndexByte(host, ':')
	square := strings.LastIndexByte(host, ']')
	if colon > square { // there a port number
		port := host[colon+1:]
		scheme := strings.ToLower(req.URL.Scheme)
		if port == "" || (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
			host = host[:colon]
		} else {
			return host
		}
	}

	// not IPv6 literal
	if square == -1 || host[0] != '[' {
		return host
	}

	return host
}

func (ctx *SigningContext) needHoisting(header string) bool {
	if (!ctx.isPresign) || (ctx.headerHoisting == nil) {
		return false
	}
	if ctx.headerHoisting.disallowed[header] {
		return false
	}
	for _, prefix := range ctx.headerHoisting.DisallowedPrefix {
		if strings.HasPrefix(header, prefix) {
			return false
		}
	}
	if len(ctx.headerHoisting.AllowedPrefix) == 0 {
		return true
	}
	for _, prefix := range ctx.headerHoisting.AllowedPrefix {
		if strings.HasPrefix(header, prefix) {
			return true
		}
	}
	return false
}

func (ctx *SigningContext) buildCanonicalHeaders(req *http.Request) {
	type pair struct {
		Name  string
		Value string
	}

	headers := make([]pair, 0, len(req.Header)+1)
	headers = append(headers, pair{
		Name:  hostHeader,
		Value: getHost(req),
	})

	for k, v := range req.Header {
		if ctx.ignoredHeaders[k] {
			continue
		}

		if ctx.needHoisting(k) {
			ctx.Query[k] = v
		} else {
			headers = append(headers, pair{
				Name:  strings.ToLower(k),
				Value: buildCanonicalHeaderValue(v),
			})
		}
	}

	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Name < headers[j].Name
	})

	var bufName, bufHeader bytes.Buffer
	for i, h := range headers {
		if i > 0 {
			bufName.WriteByte(';')
		}
		bufName.WriteString(h.Name)

		bufHeader.WriteString(h.Name)
		bufHeader.WriteByte(':')
		bufHeader.WriteString(h.Value)
		bufHeader.WriteByte('\n')
	}

	ctx.SignedHeaders = bufName.String()
	ctx.CanonicalHeaders = bufHeader.String()
}

func (ctx *SigningContext) hashBody(req *http.Request, verify bool) error {
	// we cannot use body hash in header during verify, otherwise, an attacker
	// will use an existing body hash with a new body
	if !verify {
		ctx.BodyHash = req.Header.Get(ctx.literal.ContentSHA256)
		if ctx.BodyHash != "" {
			return nil
		}
	}

	if ctx.excludeBody {
		ctx.BodyHash = unsignedPayload
		if !verify {
			req.Header.Set(ctx.literal.ContentSHA256, ctx.BodyHash)
		}
	} else if req.Body == nil {
		// sha256 of empty string
		ctx.BodyHash = sha256Empty
	} else {
		body, e := ioutil.ReadAll(req.Body)
		if e != nil {
			return e
		}
		req.Body.Close()
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
		ctx.BodyHash = sha256DegistAndEncodeToHexString(body)
	}

	return nil
}

func (ctx *SigningContext) hashCanonicalRequest(req *http.Request) string {
	var buf bytes.Buffer

	buf.WriteString(req.Method)
	buf.WriteByte('\n')

	buf.WriteString(buildCanonicalURI(req.URL))
	buf.WriteByte('\n')

	buf.WriteString(ctx.getCanonicalQuery(req.URL))
	buf.WriteByte('\n')

	buf.WriteString(ctx.CanonicalHeaders)
	buf.WriteByte('\n')

	buf.WriteString(ctx.SignedHeaders)
	buf.WriteByte('\n')

	buf.WriteString(ctx.BodyHash)

	return sha256DegistAndEncodeToHexString(buf.Bytes())
}

// sign calculate the signature of the request, but does not modify it
func (ctx *SigningContext) sign(req *http.Request) {
	var buf bytes.Buffer

	buf.WriteString(ctx.literal.AlgorithmValue)
	buf.WriteByte('\n')

	buf.WriteString(formatTime(ctx.Time))
	buf.WriteByte('\n')

	buf.WriteString(ctx.scopeString)
	buf.WriteByte('\n')

	hcr := ctx.hashCanonicalRequest(req)
	buf.WriteString(hcr)

	key := ctx.deriveSigningKey()
	hash := hmacDigest(key, buf.Bytes())
	ctx.Signature = hex.EncodeToString(hash)
}

func (ctx *SigningContext) prepare(req *http.Request) error {
	ctx.buildScopeString()
	ctx.Query = req.URL.Query()
	if e := ctx.hashBody(req, false); e != nil {
		return e
	}
	if !ctx.isPresign {
		req.Header.Set(ctx.literal.Date, formatTime(ctx.Time))
	}
	ctx.buildCanonicalHeaders(req)
	return nil
}

// Sign calculate the signature and add it to request header
func (ctx *SigningContext) Sign(req *http.Request) error {
	if e := ctx.prepare(req); e != nil {
		return e
	}
	ctx.sign(req)

	sig := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		ctx.literal.AlgorithmValue,
		ctx.AccessKeyID,
		ctx.scopeString,
		ctx.SignedHeaders,
		ctx.Signature,
	)
	req.Header.Set(authHeader, sig)

	return nil
}

// Presign calculate the signature and add it to request url
func (ctx *SigningContext) Presign(req *http.Request, expireTime time.Duration) error {
	ctx.isPresign = true
	ctx.ExpireTime = expireTime

	if e := ctx.prepare(req); e != nil {
		return e
	}
	ctx.sign(req)

	ctx.Query.Add(ctx.literal.Signature, ctx.Signature)
	req.URL.RawQuery = strings.Replace(ctx.Query.Encode(), "+", "%20", -1)
	return nil
}

func (ctx *SigningContext) initFromHeader(req *http.Request) error {
	const invalidHeaderFormat = "invalid header format: %s"

	hdr := req.Header.Get(authHeader)
	idx := strings.IndexByte(hdr, ' ')
	if idx == -1 {
		return fmt.Errorf(invalidHeaderFormat, authHeader)
	}

	if hdr[:idx] != ctx.literal.AlgorithmValue {
		return fmt.Errorf(invalidHeaderFormat, authHeader)
	}

	parts := strings.Split(hdr[idx+1:], ",")
	if len(parts) != 3 {
		return fmt.Errorf(invalidHeaderFormat, authHeader)
	}

	str := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(str, "Credential=") {
		return fmt.Errorf(invalidHeaderFormat, authHeader)
	}
	scopes := strings.Split(str[11:], "/")
	if len(scopes) < 3 {
		return fmt.Errorf(invalidHeaderFormat, authHeader)
	}
	ctx.AccessKeyID = scopes[0]
	ctx.Scopes = scopes[2 : len(scopes)-1]

	str = strings.TrimSpace(parts[1])
	if !strings.HasPrefix(str, "SignedHeaders=") {
		return fmt.Errorf(invalidHeaderFormat, authHeader)
	}
	ctx.SignedHeaders = str[14:]

	str = strings.TrimSpace(parts[2])
	if !strings.HasPrefix(str, "Signature=") {
		return fmt.Errorf(invalidHeaderFormat, authHeader)
	}
	ctx.Signature = str[10:]

	hdr = req.Header.Get(ctx.literal.Date)
	if !strings.HasPrefix(hdr, scopes[1]) {
		return fmt.Errorf("signature timestamp mismatch")
	} else if t, e := time.ParseInLocation(timeFormat, hdr, time.UTC); e != nil {
		return fmt.Errorf(invalidHeaderFormat, ctx.literal.Date)
	} else {
		ctx.Time = t
	}

	return nil
}

func (ctx *SigningContext) initFromQuery(req *http.Request) error {
	const invalidQuery = "invalid query value: %s"
	ctx.isPresign = true

	if ctx.Query.Get(ctx.literal.AlgorithmName) != ctx.literal.AlgorithmValue {
		return fmt.Errorf(invalidQuery, ctx.literal.AlgorithmName)
	}

	str := ctx.Query.Get(ctx.literal.Credential)
	scopes := strings.Split(str, "/")
	if len(scopes) < 3 {
		return fmt.Errorf(invalidQuery, ctx.literal.Credential)
	}
	ctx.AccessKeyID = scopes[0]
	ctx.Scopes = scopes[2 : len(scopes)-1]

	str = ctx.Query.Get(ctx.literal.Date)
	if !strings.HasPrefix(str, scopes[1]) {
		return fmt.Errorf("signature timestamp mismatch")
	} else if t, e := time.ParseInLocation(timeFormat, str, time.UTC); e != nil {
		return fmt.Errorf(invalidQuery, ctx.literal.Date)
	} else {
		ctx.Time = t
	}

	str = ctx.Query.Get(ctx.literal.Expires)
	v, e := strconv.ParseUint(str, 0, 64)
	if e != nil {
		return fmt.Errorf(invalidQuery, ctx.literal.Expires)
	}
	ctx.ExpireTime = time.Duration(v) * time.Second

	ctx.SignedHeaders = ctx.Query.Get(ctx.literal.SignedHeaders)
	ctx.Signature = ctx.Query.Get(ctx.literal.Signature)

	return nil
}

func (ctx *SigningContext) initFromSignedRequest(req *http.Request) error {
	ctx.Query = req.URL.Query()

	if req.Header.Get(authHeader) != "" {
		if e := ctx.initFromHeader(req); e != nil {
			return e
		}
	} else {
		if e := ctx.initFromQuery(req); e != nil {
			return e
		}
	}

	ctx.buildScopeString()

	// rebuild canonical header from signed headers
	var buf bytes.Buffer
	names := strings.Split(ctx.SignedHeaders, ";")
	for _, name := range names {
		buf.WriteString(name)
		buf.WriteByte(':')
		if name == hostHeader {
			v := getHost(req)
			buf.WriteString(v)
		} else {
			name = textproto.CanonicalMIMEHeaderKey(name)
			v := buildCanonicalHeaderValue(req.Header[name])
			buf.WriteString(v)
		}
		buf.WriteByte('\n')
	}

	ctx.CanonicalHeaders = buf.String()

	return nil
}

// Verify verifies the signature of a request
func (signer *Signer) Verify(req *http.Request) error {
	if signer.accessKeyStore == nil {
		panic("access key store must be set before calling Verify")
	}

	ctx := &SigningContext{Signer: signer}
	if e := ctx.initFromSignedRequest(req); e != nil {
		return e
	}

	age := time.Now().Sub(ctx.Time)
	if ctx.ttl > 0 {
		if age < -ctx.ttl || age > ctx.ttl {
			return fmt.Errorf("signature expired")
		}
	}
	if ctx.isPresign {
		if age > ctx.ExpireTime {
			return fmt.Errorf("signature expired")
		}
	}

	secret, ok := signer.accessKeyStore.GetSecret(ctx.AccessKeyID)
	if !ok {
		return fmt.Errorf("access-key-id not found")
	}
	ctx.AccessKeySecret = secret

	sig := ctx.Signature
	if e := ctx.hashBody(req, true); e != nil {
		return e
	}

	ctx.sign(req)
	if sig != ctx.Signature {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}
