package proxies

import "github.com/megaease/easegress/pkg/protocols"

// LoadBalancer is the interface of a load balancer.
type LoadBalancer[Request protocols.Request, Response protocols.Response] interface {
	ChooseServer(req Request) *Server
	ReturnServer(server *Server, req Request, resp Response)
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
	// Interval is the interval duration for health check.
	Interval string `json:"interval" jsonschema:"omitempty,format=duration"`
	// Path is the health check path for server
	Path string `json:"path" jsonschema:"omitempty"`
	// Timeout is the timeout duration for health check, default is 3.
	Timeout string `json:"timeout" jsonschema:"omitempty,format=duration"`
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

/*
// BaseLoadBalancer implement the common part of load balancer.
type BaseLoadBalancer[TRequest Request, TResponse Response] struct {
	spec           *LoadBalanceSpec
	Servers        []*Server
	healthyServers atomic.Value
	consistentHash *consistent.Consistent
	cookieExpire   time.Duration
	done           chan bool
	probeClient    *http.Client
	probeInterval  time.Duration
	probeTimeout   time.Duration
}

// HealthyServers return healthy servers
func (blb *BaseLoadBalancer[TRequest, TResponse]) HealthyServers() []*Server {
	return blb.healthyServers.Load().([]*Server)
}

// init initializes load balancer
func (blb *BaseLoadBalancer[TRequest, TResponse]) init(spec *LoadBalanceSpec, servers []*Server) {
	blb.spec = spec
	blb.Servers = servers
	blb.healthyServers.Store(servers)

	blb.initStickySession(spec.StickySession, blb.HealthyServers())
	blb.initHealthCheck(spec.HealthCheck, servers)
}

// initStickySession initializes for sticky session
func (blb *BaseLoadBalancer[TRequest, TResponse]) initStickySession(spec *StickySessionSpec, servers []*Server) {
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
func (blb *BaseLoadBalancer[TRequest, TResponse]) initHealthCheck(spec *HealthCheckSpec, servers []*Server) {
	if spec == nil || len(servers) == 0 {
		return
	}

	blb.probeInterval, _ = time.ParseDuration(spec.Interval)
	if blb.probeInterval <= 0 {
		blb.probeInterval = HealthCheckDefaultInterval
	}
	blb.probeTimeout, _ = time.ParseDuration(spec.Timeout)
	if blb.probeTimeout <= 0 {
		blb.probeTimeout = HealthCheckDefaultTimeout
	}
	if spec.Fails == 0 {
		spec.Fails = HealthCheckDefaultFailThreshold
	}
	if spec.Passes == 0 {
		spec.Passes = HealthCheckDefaultPassThreshold
	}
	blb.probeClient = &http.Client{Timeout: blb.probeTimeout}
	ticker := time.NewTicker(blb.probeInterval)
	blb.done = make(chan bool)
	go func() {
		for {
			select {
			case <-blb.done:
				ticker.Stop()
				return
			case <-ticker.C:
				blb.probeServers()
			}
		}
	}()
}

// probeServers checks health status of servers
func (blb *BaseLoadBalancer[TRequest, TResponse]) probeServers() {
	statusChange := false
	healthyServers := make([]*Server, 0, len(blb.Servers))
	for _, svr := range blb.Servers {
		pass := blb.probeHTTP(svr.URL)
		healthy, change := svr.RecordHealth(pass, blb.spec.HealthCheck.Passes, blb.spec.HealthCheck.Fails)
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
func (blb *BaseLoadBalancer[TRequest, TResponse]) probeHTTP(url string) bool {
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
func (blb *BaseLoadBalancer[TRequest, TResponse]) initConsistentHash() {
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
func (blb *BaseLoadBalancer[TRequest, TResponse]) configLBCookie() {
	if blb.spec.StickySession.LBCookieName == "" {
		blb.spec.StickySession.LBCookieName = StickySessionDefaultLBCookieName
	}

	blb.cookieExpire, _ = time.ParseDuration(blb.spec.StickySession.LBCookieExpire)
	if blb.cookieExpire <= 0 {
		blb.cookieExpire = StickySessionDefaultLBCookieExpire
	}
}

// ChooseServer chooses the sticky server if enable
func (blb *BaseLoadBalancer[TRequest, TResponse]) ChooseServer(req *httpprot.Request) *Server {
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
func (blb *BaseLoadBalancer[TRequest, TResponse]) chooseServerByConsistentHash(req *httpprot.Request) *Server {
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
func (blb *BaseLoadBalancer[TRequest, TResponse]) chooseServerByLBCookie(req *httpprot.Request) *Server {
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
func (blb *BaseLoadBalancer[TRequest, TResponse]) ReturnServer(server *Server, req *httpprot.Request, resp *httpprot.Response) {
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
func (blb *BaseLoadBalancer[TRequest, TResponse]) Close() {
	if blb.done != nil {
		close(blb.done)
	}
}

*/
