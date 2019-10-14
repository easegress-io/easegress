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
	policyRoundRobin     = "roundRobin"
	policyRandom         = "random"
	policyWeightedRandom = "weightedRandom"
	policyIPHash         = "ipHash"
	policyHeaderHash     = "headerHash"
)

type (
	servers struct {
		count      uint64
		weightsSum int
		servers    []*server
		lb         *loadBalance
	}

	// server is backend server.
	server struct {
		URL    string   `yaml:"url" v:"required,url"`
		Tags   []string `yaml:"tags" v:"unique,dive,required"`
		Weight int      `yaml:"weight" v:"gte=0,lte=100"`
	}

	// loadBalance is load balance for multiple servers.
	loadBalance struct {
		V string `yaml:"-" v:"parent"`

		Policy        string `yaml:"policy" v:"required,oneof=roundRobin random weightedRandom ipHash headerHash"`
		HeaderHashKey string `yaml:"headerHashKey"`
	}
)

func (s *server) String() string {
	return fmt.Sprintf("%s,%v,%d", s.URL, s.Tags, s.Weight)
}

// Validate validates LoadBalance.
func (lb loadBalance) Validate() error {
	if lb.Policy == policyHeaderHash && len(lb.HeaderHashKey) == 0 {
		return fmt.Errorf("headerHash needs to speficy headerHashKey")
	}

	return nil
}

func newServers(spec *poolSpec) *servers {
	s := &servers{
		lb: spec.LoadBalance,
	}
	defer s.prepare()

	if len(spec.ServersTags) == 0 {
		s.servers = spec.Servers
		return s
	}

	servers := make([]*server, 0)
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

func (s *servers) next(ctx context.HTTPContext) *server {
	switch s.lb.Policy {
	case policyRoundRobin:
		return s.roundRobin(ctx)
	case policyRandom:
		return s.random(ctx)
	case policyWeightedRandom:
		return s.weightedRandom(ctx)
	case policyIPHash:
		return s.ipHash(ctx)
	case policyHeaderHash:
		return s.headerHash(ctx)
	}

	logger.Errorf("BUG: unknown load balance policy: %s", s.lb.Policy)

	return s.roundRobin(ctx)
}

func (s *servers) roundRobin(ctx context.HTTPContext) *server {
	count := atomic.AddUint64(&s.count, 1)
	return s.servers[int(count)%len(s.servers)]
}

func (s *servers) random(ctx context.HTTPContext) *server {
	return s.servers[rand.Intn(len(s.servers))]
}

func (s *servers) weightedRandom(ctx context.HTTPContext) *server {
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

func (s *servers) ipHash(ctx context.HTTPContext) *server {
	sum32 := int(hashtool.Hash32(ctx.Request().RealIP()))
	return s.servers[sum32%len(s.servers)]
}

func (s *servers) headerHash(ctx context.HTTPContext) *server {
	value := ctx.Request().Header().Get(s.lb.HeaderHashKey)
	sum32 := int(hashtool.Hash32(value))
	return s.servers[sum32%len(s.servers)]
}
