package backend

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/serviceregistry"
	"github.com/megaease/easegateway/pkg/util/hashtool"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	// PolicyRoundRobin is the policy of round robin.
	PolicyRoundRobin = "roundRobin"
	// PolicyRandom is the policy of random.
	PolicyRandom = "random"
	// PolicyWeightedRandom is the policy of weighted random.
	PolicyWeightedRandom = "weightedRandom"
	// PolicyIPHash is the policy of ip hash.
	PolicyIPHash = "ipHash"
	// PolicyHeaderHash is the policy of header hash.
	PolicyHeaderHash = "headerHash"

	retryTimeout = 3 * time.Second
)

type (
	servers struct {
		poolSpec *PoolSpec

		mutex   sync.Mutex
		service *serviceregistry.Service
		static  *staticServers
		done    chan struct{}
	}

	staticServers struct {
		count      uint64
		weightsSum int
		servers    []*Server
		lb         LoadBalance
	}

	// Server is backend server.
	Server struct {
		URL    string   `yaml:"url" jsonschema:"required,format=url"`
		Tags   []string `yaml:"tags" jsonschema:"omitempty,uniqueItems=true"`
		Weight int      `yaml:"weight" jsonschema:"omitempty,minimum=0,maximum=100"`
	}

	// LoadBalance is load balance for multiple servers.
	LoadBalance struct {
		Policy        string `yaml:"policy" jsonschema:"required,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
		HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
	}
)

func (s *Server) String() string {
	return fmt.Sprintf("%s,%v,%d", s.URL, s.Tags, s.Weight)
}

// Validate validates LoadBalance.
func (lb LoadBalance) Validate() error {
	if lb.Policy == PolicyHeaderHash && len(lb.HeaderHashKey) == 0 {
		return fmt.Errorf("headerHash needs to speficy headerHashKey")
	}

	return nil
}

func newServers(poolSpec *PoolSpec) *servers {
	s := &servers{
		poolSpec: poolSpec,
		done:     make(chan struct{}),
	}

	s.tryUpdateService()

	go s.run()

	return s
}

func (s *servers) run() {
	if s.poolSpec.ServiceName == "" {
		return
	}

	for {
		service := s.mustUpdateService()

		// NOTE: The servers is closed.
		if service == nil {
			return
		}

		select {
		// NOTE: Defensive programming.
		case <-s.done:
			return
		case <-service.Updated():
			logger.Infof("service %s updated, try to update",
				s.poolSpec.ServiceName)
		case <-service.Closed():
			logger.Warnf("service %s closed: %s, try to get again",
				s.poolSpec.ServiceName, service.CloseMessage())
		}
	}
}

// mustUpdateService blocks until getting the service or closed.
func (s *servers) mustUpdateService() *serviceregistry.Service {
	for {
		service, err := s.useService()
		if err == nil {
			return service
		}
		logger.Warnf("%v", err)
		select {
		case <-s.done:
			return nil
		case <-time.After(retryTimeout):
		}
	}
}

// tryUpdateService uses static servers if it failed to get service.
func (s *servers) tryUpdateService() {
	if s.poolSpec.ServiceName == "" {
		s.useStaticServers()
		return
	}

	_, err := s.useService()
	if err == nil {
		return
	}

	logger.Errorf("%v", err)
	if len(s.poolSpec.Servers) > 0 {
		logger.Warnf("fallback to static severs")
		s.useStaticServers()
	} else {
		logger.Warnf("no static server available either")
	}
}

func (s *servers) useStaticServers() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.static = newStaticServers(s.poolSpec.Servers,
		s.poolSpec.ServersTags,
		*s.poolSpec.LoadBalance)
	s.service = nil
}

func (s *servers) useService() (*serviceregistry.Service, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	service, err := serviceregistry.Global.GetService(s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("get service %s failed: %v", s.poolSpec.ServiceName, err)
	}

	var serversInput []*Server
	servers := service.Servers()
	for _, snapshotServer := range servers {
		serversInput = append(serversInput, &Server{
			URL:    snapshotServer.URL(),
			Tags:   snapshotServer.Tags,
			Weight: snapshotServer.Weight,
		})
	}
	static := newStaticServers(serversInput, s.poolSpec.ServersTags, *s.poolSpec.LoadBalance)

	s.static, s.service = static, service

	return service, nil
}

func (s *servers) snapshot() (*staticServers, *serviceregistry.Service) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.static, s.service
}

func (s *servers) len() int {
	static, _ := s.snapshot()

	if static == nil {
		return 0
	}

	return static.len()
}

func (s *servers) next(ctx context.HTTPContext) (*Server, error) {
	static, _ := s.snapshot()

	if static.len() == 0 {
		return nil, fmt.Errorf("no server available")
	}

	return static.next(ctx), nil
}

func (s *servers) close() {
	close(s.done)
}

func newStaticServers(servers []*Server, tags []string, lb LoadBalance) *staticServers {
	ss := &staticServers{
		lb: lb,
	}
	defer ss.prepare()

	if len(tags) == 0 {
		ss.servers = servers
		return ss
	}

	chosenServers := make([]*Server, 0)
	for _, server := range servers {
		for _, tag := range tags {
			if stringtool.StrInSlice(tag, server.Tags) {
				chosenServers = append(chosenServers, server)
				break
			}
		}
	}
	ss.servers = chosenServers

	return ss
}

func (ss *staticServers) prepare() {
	for _, server := range ss.servers {
		ss.weightsSum += server.Weight
	}
}

func (ss *staticServers) len() int {
	return len(ss.servers)
}

func (ss *staticServers) next(ctx context.HTTPContext) *Server {
	switch ss.lb.Policy {
	case PolicyRoundRobin:
		return ss.roundRobin(ctx)
	case PolicyRandom:
		return ss.random(ctx)
	case PolicyWeightedRandom:
		return ss.weightedRandom(ctx)
	case PolicyIPHash:
		return ss.ipHash(ctx)
	case PolicyHeaderHash:
		return ss.headerHash(ctx)
	}

	logger.Errorf("BUG: unknown load balance policy: %s", ss.lb.Policy)

	return ss.roundRobin(ctx)
}

func (ss *staticServers) roundRobin(ctx context.HTTPContext) *Server {
	count := atomic.AddUint64(&ss.count, 1)
	// NOTE: start from 0.
	count--
	return ss.servers[int(count)%len(ss.servers)]
}

func (ss *staticServers) random(ctx context.HTTPContext) *Server {
	return ss.servers[rand.Intn(len(ss.servers))]
}

func (ss *staticServers) weightedRandom(ctx context.HTTPContext) *Server {
	randomWeight := rand.Intn(ss.weightsSum)
	for _, server := range ss.servers {
		randomWeight -= server.Weight
		if randomWeight < 0 {
			return server
		}
	}

	logger.Errorf("BUG: weighted random can't pick a server: sum(%d) servers(%+v)",
		ss.weightsSum, ss.servers)

	return ss.random(ctx)
}

func (ss *staticServers) ipHash(ctx context.HTTPContext) *Server {
	sum32 := int(hashtool.Hash32(ctx.Request().RealIP()))
	return ss.servers[sum32%len(ss.servers)]
}

func (ss *staticServers) headerHash(ctx context.HTTPContext) *Server {
	value := ctx.Request().Header().Get(ss.lb.HeaderHashKey)
	sum32 := int(hashtool.Hash32(value))
	return ss.servers[sum32%len(ss.servers)]
}
