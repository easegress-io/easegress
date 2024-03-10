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

package proxies

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash/maphash"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/spaolacci/murmur3"
)

const (
	// StickySessionModeCookieConsistentHash is the sticky session mode of consistent hash on app cookie.
	StickySessionModeCookieConsistentHash = "CookieConsistentHash"
	// StickySessionModeDurationBased uses a load balancer-generated cookie for stickiness.
	StickySessionModeDurationBased = "DurationBased"
	// StickySessionModeApplicationBased uses a load balancer-generated cookie depends on app cookie for stickiness.
	StickySessionModeApplicationBased = "ApplicationBased"

	// StickySessionDefaultLBCookieName is the default name of the load balancer-generated cookie.
	StickySessionDefaultLBCookieName = "EG_SESSION"

	// KeyLen is the key length used by HMAC.
	KeyLen = 8
)

// StickySessionSpec is the spec for sticky session.
type StickySessionSpec struct {
	Mode string `json:"mode" jsonschema:"required,enum=CookieConsistentHash,enum=DurationBased,enum=ApplicationBased"`
	// AppCookieName is the user-defined cookie name in CookieConsistentHash and ApplicationBased mode.
	AppCookieName string `json:"appCookieName,omitempty"`
	// LBCookieName is the generated cookie name in DurationBased and ApplicationBased mode.
	LBCookieName string `json:"lbCookieName,omitempty"`
	// LBCookieExpire is the expire seconds of generated cookie in DurationBased and ApplicationBased mode.
	LBCookieExpire string `json:"lbCookieExpire,omitempty" jsonschema:"format=duration"`
}

// SessionSticker is the interface for session stickiness.
type SessionSticker interface {
	UpdateServers(servers []*Server)
	GetServer(req protocols.Request, sg *ServerGroup) *Server
	ReturnServer(server *Server, req protocols.Request, resp protocols.Response)
	Close()
}

// hashMember is member used for hash
type hashMember struct {
	server *Server
}

// String implements consistent.Member interface
func (m hashMember) String() string {
	return m.server.ID()
}

// hasher is used for hash
type hasher struct{}

// Sum64 implement hash function using murmur3
func (h hasher) Sum64(data []byte) uint64 {
	return murmur3.Sum64(data)
}

// HTTPSessionSticker implements sticky session for HTTP.
type HTTPSessionSticker struct {
	spec           *StickySessionSpec
	consistentHash atomic.Pointer[consistent.Consistent]
	cookieExpire   time.Duration
}

// NewHTTPSessionSticker creates a new HTTPSessionSticker.
func NewHTTPSessionSticker(spec *StickySessionSpec) SessionSticker {
	if spec.LBCookieName == "" {
		spec.LBCookieName = StickySessionDefaultLBCookieName
	}

	ss := &HTTPSessionSticker{spec: spec}

	ss.cookieExpire, _ = time.ParseDuration(spec.LBCookieExpire)
	if ss.cookieExpire <= 0 {
		ss.cookieExpire = time.Hour * 2
	}

	return ss
}

// UpdateServers update the servers for the HTTPSessionSticker.
func (ss *HTTPSessionSticker) UpdateServers(servers []*Server) {
	if ss.spec.Mode != StickySessionModeCookieConsistentHash {
		return
	}

	if len(servers) == 0 {
		// TODO: consistentHash panics in this case, we need to handle it.
		return
	}

	members := make([]consistent.Member, len(servers))
	for i, s := range servers {
		members[i] = hashMember{server: s}
	}

	cfg := consistent.Config{
		PartitionCount:    1024,
		ReplicationFactor: 50,
		Load:              1.25,
		Hasher:            hasher{},
	}

	ss.consistentHash.Store(consistent.New(members, cfg))
}

func (ss *HTTPSessionSticker) getServerByConsistentHash(req *httpprot.Request) *Server {
	cookie, err := req.Cookie(ss.spec.AppCookieName)
	if err != nil {
		return nil
	}

	m := ss.consistentHash.Load().LocateKey([]byte(cookie.Value))
	if m != nil {
		return m.(hashMember).server
	}

	return nil
}

func (ss *HTTPSessionSticker) getServerByLBCookie(req *httpprot.Request, sg *ServerGroup) *Server {
	cookie, err := req.Cookie(ss.spec.LBCookieName)
	if err != nil {
		return nil
	}

	signed, err := hex.DecodeString(cookie.Value)
	if err != nil || len(signed) != KeyLen+sha256.Size {
		return nil
	}

	key := signed[:KeyLen]
	macBytes := signed[KeyLen:]
	for _, s := range sg.Servers {
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(s.ID()))
		expected := mac.Sum(nil)
		if hmac.Equal(expected, macBytes) {
			return s
		}
	}

	return nil
}

// GetServer returns the server for the request.
func (ss *HTTPSessionSticker) GetServer(req protocols.Request, sg *ServerGroup) *Server {
	httpreq, ok := req.(*httpprot.Request)
	if !ok {
		panic("not http request")
	}

	switch ss.spec.Mode {
	case StickySessionModeCookieConsistentHash:
		return ss.getServerByConsistentHash(httpreq)
	case StickySessionModeDurationBased, StickySessionModeApplicationBased:
		return ss.getServerByLBCookie(httpreq, sg)
	}

	return nil
}

// sign signs plain text byte array to encoded string
func sign(plain []byte) string {
	signed := make([]byte, KeyLen+sha256.Size)
	key := signed[:KeyLen]
	macBytes := signed[KeyLen:]

	// use maphash to generate random key fast
	binary.LittleEndian.PutUint64(key, new(maphash.Hash).Sum64())
	mac := hmac.New(sha256.New, key)
	mac.Write(plain)
	mac.Sum(macBytes[:0])

	return hex.EncodeToString(signed)
}

// ReturnServer returns the server to the session sticker.
func (ss *HTTPSessionSticker) ReturnServer(server *Server, req protocols.Request, resp protocols.Response) {
	httpresp, ok := resp.(*httpprot.Response)
	if !ok {
		panic("not http response")
	}

	setCookie := false
	switch ss.spec.Mode {
	case StickySessionModeDurationBased:
		setCookie = true
	case StickySessionModeApplicationBased:
		for _, c := range httpresp.Cookies() {
			if c.Name == ss.spec.AppCookieName {
				setCookie = true
				break
			}
		}
	}

	if setCookie {
		cookie := &http.Cookie{
			Name:    ss.spec.LBCookieName,
			Value:   sign([]byte(server.ID())),
			Expires: time.Now().Add(ss.cookieExpire),
		}
		httpresp.SetCookie(cookie)
	}
}

// Close closes the HTTPSessionSticker.
func (ss *HTTPSessionSticker) Close() {
}
