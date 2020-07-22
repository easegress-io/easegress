package backend

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/hashtool"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	PolicyRoundRobin     = "roundRobin"
	PolicyRandom         = "random"
	PolicyWeightedRandom = "weightedRandom"
	PolicyIPHash         = "ipHash"
	PolicyHeaderHash     = "headerHash"
)

type (
	servers struct {
		count      uint64
		weightsSum int
		servers    []*Server
		lb         *LoadBalance
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

func newServers(spec *PoolSpec) *servers {
	s := &servers{
		lb: spec.LoadBalance,
	}
	defer s.prepare()

	if len(spec.ServersTags) == 0 {
		s.servers = spec.Servers
		return s
	}

	servers := make([]*Server, 0)
	for _, server := range spec.Servers {
		for _, tag := range spec.ServersTags {
			if stringtool.StrInSlice(tag, server.Tags) {
				servers = append(servers, server)
				break
			}
		}
	}
	s.servers = servers

	return s
}

func (s *servers) prepare() {
	for _, server := range s.servers {
		s.weightsSum += server.Weight
	}
}

func (s *servers) len() int {
	return len(s.servers)
}

func (s *servers) next(ctx context.HTTPContext) *Server {
	switch s.lb.Policy {
	case PolicyRoundRobin:
		return s.roundRobin(ctx)
	case PolicyRandom:
		return s.random(ctx)
	case PolicyWeightedRandom:
		return s.weightedRandom(ctx)
	case PolicyIPHash:
		return s.ipHash(ctx)
	case PolicyHeaderHash:
		return s.headerHash(ctx)
	}

	logger.Errorf("BUG: unknown load balance policy: %s", s.lb.Policy)

	return s.roundRobin(ctx)
}

func (s *servers) roundRobin(ctx context.HTTPContext) *Server {
	count := atomic.AddUint64(&s.count, 1)
	return s.servers[int(count)%len(s.servers)]
}

func (s *servers) random(ctx context.HTTPContext) *Server {
	return s.servers[rand.Intn(len(s.servers))]
}

func (s *servers) weightedRandom(ctx context.HTTPContext) *Server {
	randomWeight := rand.Intn(s.weightsSum)
	for _, server := range s.servers {
		randomWeight -= server.Weight
		if randomWeight < 0 {
			return server
		}
	}

	logger.Errorf("BUG: weighted random can't pick a server: sum(%d) servers(%+v)",
		s.weightsSum, s.servers)

	return s.random(ctx)
}

func (s *servers) ipHash(ctx context.HTTPContext) *Server {
	sum32 := int(hashtool.Hash32(ctx.Request().RealIP()))
	return s.servers[sum32%len(s.servers)]
}

func (s *servers) headerHash(ctx context.HTTPContext) *Server {
	value := ctx.Request().Header().Get(s.lb.HeaderHashKey)
	sum32 := int(hashtool.Hash32(value))
	return s.servers[sum32%len(s.servers)]
}
