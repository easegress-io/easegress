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

package proxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"hash/maphash"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/spaolacci/murmur3"
)

const (
	// LoadBalancePolicyRoundRobin is the load balance policy of round robin.
	LoadBalancePolicyRoundRobin = "roundRobin"
	// LoadBalancePolicyRandom is the load balance policy of random.
	LoadBalancePolicyRandom = "random"
	// LoadBalancePolicyWeightedRandom is the load balance policy of weighted random.
	LoadBalancePolicyWeightedRandom = "weightedRandom"
	// LoadBalancePolicyIPHash is the load balance policy of IP hash.
	LoadBalancePolicyIPHash = "ipHash"
	// LoadBalancePolicyHeaderHash is the load balance policy of HTTP header hash.
	LoadBalancePolicyHeaderHash = "headerHash"
	// StickySessionModeCookieConsistentHash is the sticky session mode of consistent hash on app cookie.
	StickySessionModeCookieConsistentHash = "CookieConsistentHash"
	// StickySessionModeDurationBased uses a load balancer-generated cookie for stickiness.
	StickySessionModeDurationBased = "DurationBased"
	// StickySessionModeApplicationBased uses a load balancer-generated cookie depends on app cookie for stickiness.
	StickySessionModeApplicationBased = "ApplicationBased"
	// StickySessionDefaultLBCookieName is the default name of the load balancer-generated cookie.
	StickySessionDefaultLBCookieName = "EG_SESSION"
	// StickySessionDefaultLBCookieExpire is the default expiration duration of the load balancer-generated cookie.
	StickySessionDefaultLBCookieExpire = time.Hour * 2
	// KeyLen is the key length used by HMAC.
	KeyLen = 8
	// HealthCheckDefaultTimeout is the default timeout for health check
	HealthCheckDefaultTimeout = 3
	// HealthCheckDefaultFailThreshold is the default fail threshold for health check
	HealthCheckDefaultFailThreshold = 1
	// HealthCheckDefaultPassThreshold is the default pass threshold for health check
	HealthCheckDefaultPassThreshold = 1
)

// LoadBalancer is the interface of an HTTP load balancer.
type LoadBalancer interface {
	ChooseServer(req *httpprot.Request) *Server
	ReturnServer(server *Server, req *httpprot.Request, resp *httpprot.Response)
	HealthyServers() []*Server
	Close()
}

// StickySessionSpec is the spec for sticky session.
type StickySessionSpec struct {
	Mode string `json:"mode" jsonschema:"required,enum=CookieConsistentHash,enum=DurationBased,enum=ApplicationBased"`
	// AppCookieName is the user-defined cookie name in CookieConsistentHash and ApplicationBased mode.
	AppCookieName string `json:"appCookieName" jsonschema:"omitempty"`
	// LBCookieName is the generated cookie name in DurationBased and ApplicationBased mode.
	LBCookieName string `json:"lbCookieName" jsonschema:"omitempty"`
	// LBCookieExpire is the expire seconds of generated cookie in DurationBased and ApplicationBased mode.
	LBCookieExpire string `json:"lbCookieExpire" jsonschema:"omitempty,format=duration"`
}

// HealthCheckSpec is the spec for health check.
type HealthCheckSpec struct {
	// Interval is the interval seconds for health check.
	Interval int `json:"interval" jsonschema:"omitempty,minimum=1"`
	// Path is the health check path for server
	Path string `json:"path" jsonschema:"omitempty"`
	// Timeout is the timeout seconds for health check, default is 3.
	Timeout int `json:"timeout" jsonschema:"omitempty,minimum=1"`
	// Fails is the consecutive fails count for assert fail, default is 1.
	Fails int `json:"fails" jsonschema:"omitempty,minimum=1"`
	// Passes is the consecutive passes count for assert pass, default is 1.
	Passes int `json:"passes" jsonschema:"omitempty,minimum=1"`
}

// LoadBalanceSpec is the spec to create a load balancer.
type LoadBalanceSpec struct {
	Policy        string             `json:"policy" jsonschema:"omitempty,enum=,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
	HeaderHashKey string             `json:"headerHashKey" jsonschema:"omitempty"`
	StickySession *StickySessionSpec `json:"stickySession" jsonschema:"omitempty"`
	HealthCheck   *HealthCheckSpec   `json:"healthCheck" jsonschema:"omitempty"`
}

// NewLoadBalancer creates a load balancer for servers according to spec.
func NewLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer {
	switch spec.Policy {
	case LoadBalancePolicyRoundRobin, "":
		return newRoundRobinLoadBalancer(spec, servers)
	case LoadBalancePolicyRandom:
		return newRandomLoadBalancer(spec, servers)
	case LoadBalancePolicyWeightedRandom:
		return newWeightedRandomLoadBalancer(spec, servers)
	case LoadBalancePolicyIPHash:
		return newIPHashLoadBalancer(spec, servers)
	case LoadBalancePolicyHeaderHash:
		return newHeaderHashLoadBalancer(spec, servers)
	default:
		logger.Errorf("unsupported load balancing policy: %s", spec.Policy)
		return newRoundRobinLoadBalancer(spec, servers)
	}
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

// BaseLoadBalancer implement the common part of load balancer.
type BaseLoadBalancer struct {
	spec           *LoadBalanceSpec
	Servers        []*Server
	healthyServers atomic.Value
	consistentHash *consistent.Consistent
	cookieExpire   time.Duration
	ticker         *time.Ticker
	probeClient    *http.Client
}

// HealthyServers return healthy servers
func (blb *BaseLoadBalancer) HealthyServers() []*Server {
	return blb.healthyServers.Load().([]*Server)
}

// init initializes load balancer
func (blb *BaseLoadBalancer) init(spec *LoadBalanceSpec, servers []*Server) {
	blb.spec = spec
	blb.Servers = servers
	blb.healthyServers.Store(servers)

	blb.initStickySession(spec.StickySession, blb.HealthyServers())
	blb.initHealthCheck(spec.HealthCheck, servers)
}

// initStickySession initializes for sticky session
func (blb *BaseLoadBalancer) initStickySession(spec *StickySessionSpec, servers []*Server) {
	if spec == nil || len(servers) == 0 {
		return
	}

	switch spec.Mode {
	case StickySessionModeCookieConsistentHash:
		blb.initConsistentHash()
	case StickySessionModeDurationBased, StickySessionModeApplicationBased:
		blb.configLBCookie()
	}
}

// initHealthCheck initializes for health check
func (blb *BaseLoadBalancer) initHealthCheck(spec *HealthCheckSpec, servers []*Server) {
	if spec == nil || spec.Interval == 0 || len(servers) == 0 {
		return
	}

	if spec.Timeout == 0 {
		spec.Timeout = HealthCheckDefaultTimeout
	}
	if spec.Fails == 0 {
		spec.Fails = HealthCheckDefaultFailThreshold
	}
	if spec.Passes == 0 {
		spec.Passes = HealthCheckDefaultPassThreshold
	}
	blb.probeClient = &http.Client{Timeout: time.Duration(spec.Timeout) * time.Second}
	blb.ticker = time.NewTicker(time.Duration(spec.Interval) * time.Second)
	go func(t *time.Ticker) {
		for {
			<-t.C
			blb.probeServers()
		}
	}(blb.ticker)
}

// probeServers checks health status of servers
func (blb *BaseLoadBalancer) probeServers() {
	statusChange := false
	healthyServers := make([]*Server, 0, len(blb.Servers))
	for _, svr := range blb.Servers {
		pass := blb.probeHTTP(svr.URL)
		healthy, change := svr.recordHealth(pass, blb.spec.HealthCheck.Passes, blb.spec.HealthCheck.Fails)
		if change {
			statusChange = true
		}
		if healthy {
			healthyServers = append(healthyServers, svr)
		}
	}
	if statusChange {
		blb.healthyServers.Store(healthyServers)
		// init consistent hash in sticky session when servers change
		blb.initStickySession(blb.spec.StickySession, blb.HealthyServers())
	}
}

// probeHTTP checks http url status
func (blb *BaseLoadBalancer) probeHTTP(url string) bool {
	if blb.spec.HealthCheck.Path != "" {
		url += blb.spec.HealthCheck.Path
	}
	res, err := blb.probeClient.Get(url)
	if err != nil || res.StatusCode > 500 {
		return false
	}
	return true
}

// initConsistentHash initializes for consistent hash mode
func (blb *BaseLoadBalancer) initConsistentHash() {
	members := make([]consistent.Member, len(blb.HealthyServers()))
	for i, s := range blb.HealthyServers() {
		members[i] = hashMember{server: s}
	}

	cfg := consistent.Config{
		PartitionCount:    1024,
		ReplicationFactor: 50,
		Load:              1.25,
		Hasher:            hasher{},
	}
	blb.consistentHash = consistent.New(members, cfg)
}

// configLBCookie configures properties for load balancer-generated cookie
func (blb *BaseLoadBalancer) configLBCookie() {
	if blb.spec.StickySession.LBCookieName == "" {
		blb.spec.StickySession.LBCookieName = StickySessionDefaultLBCookieName
	}

	blb.cookieExpire, _ = time.ParseDuration(blb.spec.StickySession.LBCookieExpire)
	if blb.cookieExpire <= 0 {
		blb.cookieExpire = StickySessionDefaultLBCookieExpire
	}
}

// ChooseServer chooses the sticky server if enable
func (blb *BaseLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if blb.spec.StickySession == nil {
		return nil
	}

	switch blb.spec.StickySession.Mode {
	case StickySessionModeCookieConsistentHash:
		return blb.chooseServerByConsistentHash(req)
	case StickySessionModeDurationBased, StickySessionModeApplicationBased:
		return blb.chooseServerByLBCookie(req)
	}

	return nil
}

// chooseServerByConsistentHash chooses server using consistent hash on cookie
func (blb *BaseLoadBalancer) chooseServerByConsistentHash(req *httpprot.Request) *Server {
	cookie, err := req.Cookie(blb.spec.StickySession.AppCookieName)
	if err != nil {
		return nil
	}

	m := blb.consistentHash.LocateKey([]byte(cookie.Value))
	if m != nil {
		return m.(hashMember).server
	}

	return nil
}

// chooseServerByLBCookie chooses server by load balancer-generated cookie
func (blb *BaseLoadBalancer) chooseServerByLBCookie(req *httpprot.Request) *Server {
	cookie, err := req.Cookie(blb.spec.StickySession.LBCookieName)
	if err != nil {
		return nil
	}

	signed, err := hex.DecodeString(cookie.Value)
	if err != nil || len(signed) != KeyLen+sha256.Size {
		return nil
	}

	key := signed[:KeyLen]
	macBytes := signed[KeyLen:]
	for _, s := range blb.HealthyServers() {
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(s.ID()))
		expected := mac.Sum(nil)
		if hmac.Equal(expected, macBytes) {
			return s
		}
	}

	return nil
}

// ReturnServer does some custom work before return server
func (blb *BaseLoadBalancer) ReturnServer(server *Server, req *httpprot.Request, resp *httpprot.Response) {
	if blb.spec.StickySession == nil {
		return
	}

	setCookie := false
	switch blb.spec.StickySession.Mode {
	case StickySessionModeDurationBased:
		setCookie = true
	case StickySessionModeApplicationBased:
		for _, c := range resp.Cookies() {
			if c.Name == blb.spec.StickySession.AppCookieName {
				setCookie = true
				break
			}
		}
	}
	if setCookie {
		cookie := &http.Cookie{
			Name:    blb.spec.StickySession.LBCookieName,
			Value:   sign([]byte(server.ID())),
			Expires: time.Now().Add(blb.cookieExpire),
		}
		resp.SetCookie(cookie)
	}
}

// Close closes resources
func (blb *BaseLoadBalancer) Close() {
	if blb.ticker != nil {
		blb.ticker.Stop()
	}
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

// randomLoadBalancer does load balancing in a random manner.
type randomLoadBalancer struct {
	BaseLoadBalancer
}

func newRandomLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *randomLoadBalancer {
	lb := &randomLoadBalancer{}
	lb.init(spec, servers)
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *randomLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.HealthyServers()) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	return lb.HealthyServers()[rand.Intn(len(lb.HealthyServers()))]
}

// roundRobinLoadBalancer does load balancing in a round robin manner.
type roundRobinLoadBalancer struct {
	BaseLoadBalancer
	counter uint64
}

func newRoundRobinLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *roundRobinLoadBalancer {
	lb := &roundRobinLoadBalancer{}
	lb.init(spec, servers)
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *roundRobinLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.HealthyServers()) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	counter := atomic.AddUint64(&lb.counter, 1) - 1
	return lb.HealthyServers()[int(counter)%len(lb.HealthyServers())]
}

// WeightedRandomLoadBalancer does load balancing in a weighted random manner.
type WeightedRandomLoadBalancer struct {
	BaseLoadBalancer
	totalWeight int
}

func newWeightedRandomLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *WeightedRandomLoadBalancer {
	lb := &WeightedRandomLoadBalancer{}
	lb.init(spec, servers)
	for _, server := range lb.HealthyServers() {
		lb.totalWeight += server.Weight
	}
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *WeightedRandomLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.HealthyServers()) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	randomWeight := rand.Intn(lb.totalWeight)
	for _, server := range lb.HealthyServers() {
		randomWeight -= server.Weight
		if randomWeight < 0 {
			return server
		}
	}

	panic(fmt.Errorf("BUG: should not run to here, total weight=%d", lb.totalWeight))
}

// ipHashLoadBalancer does load balancing based on IP hash.
type ipHashLoadBalancer struct {
	BaseLoadBalancer
}

func newIPHashLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *ipHashLoadBalancer {
	lb := &ipHashLoadBalancer{}
	lb.init(spec, servers)
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *ipHashLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.HealthyServers()) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	ip := req.RealIP()
	hash := fnv.New32()
	hash.Write([]byte(ip))
	return lb.HealthyServers()[hash.Sum32()%uint32(len(lb.HealthyServers()))]
}

// headerHashLoadBalancer does load balancing based on header hash.
type headerHashLoadBalancer struct {
	BaseLoadBalancer
	key string
}

func newHeaderHashLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *headerHashLoadBalancer {
	lb := &headerHashLoadBalancer{}
	lb.init(spec, servers)
	lb.key = spec.HeaderHashKey
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *headerHashLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.HealthyServers()) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	v := req.HTTPHeader().Get(lb.key)
	hash := fnv.New32()
	hash.Write([]byte(v))
	return lb.HealthyServers()[hash.Sum32()%uint32(len(lb.HealthyServers()))]
}
