package routers

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/stretchr/testify/assert"
)

func TestRuleInit(t *testing.T) {
	assert := assert.New(t)

	rule := &Rule{
		Host:       "www.megaease.com",
		HostRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			AllowIPs: []string{"192.168.1.0/24"},
		},
		Paths: []*Path{
			{
				Path: "/api/test",
			},
		},
	}

	rule.init(nil)

	assert.NotNil(rule.hostRE)
	assert.NotNil(rule.ipFilter)
	assert.NotNil(rule.ipFilterChain)
	assert.Equal(len(rule.ipFilterChain.Filters()), 1)
	assert.Equal(len(rule.Paths), 1)
	assert.NotNil(rule.Paths[0].ipFilterChain)
	assert.Equal(len(rule.Paths[0].ipFilterChain.Filters()), 1)

	rule2 := &Rule{
		Host:       "www.megaease.com",
		HostRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			AllowIPs: []string{"192.168.1.0/24"},
		},
		Paths: []*Path{
			{
				Path: "/api/test",
				IPFilterSpec: &ipfilter.Spec{
					AllowIPs: []string{"192.168.1.0/24"},
				},
			},
		},
	}

	rule2.init(ipfilter.NewIPFilterChain(nil, &ipfilter.Spec{
		AllowIPs: []string{"192.168.1.0/24"},
	}))

	assert.NotNil(rule2.hostRE)
	assert.NotNil(rule2.ipFilter)
	assert.NotNil(rule2.ipFilterChain)
	assert.Equal(len(rule2.ipFilterChain.Filters()), 2)
	assert.Equal(len(rule2.Paths), 1)
	assert.NotNil(rule2.Paths[0].ipFilterChain)
	assert.Equal(len(rule2.Paths[0].ipFilterChain.Filters()), 3)
}

func TestRuleMatch(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com:8080", nil)
	req, _ := httpprot.NewRequest(stdr)

	rule := &Rule{}
	rule.init(nil)

	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{Host: "www.megaease.com"}
	rule.init(nil)
	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{HostRegexp: `^[^.]+\.megaease\.com$`}
	rule.init(nil)
	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{HostRegexp: `^[^.]+\.megaease\.cn$`}
	rule.init(nil)
	assert.NotNil(rule)
	assert.False(rule.Match(req))
}

func TestRuleAllowIP(t *testing.T) {
	assert := assert.New(t)

	rule := &Rule{
		Host:       "www.megaease.com",
		HostRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
		Paths: []*Path{
			{
				Path: "/api/test",
			},
		},
	}

	rule.init(nil)

	assert.True(rule.AllowIP("192.168.1.1"))
	assert.False(rule.AllowIP("10.168.1.1"))

	rule.ipFilter = nil
	assert.True(rule.AllowIP("192.168.1.1"))
	assert.True(rule.AllowIP("10.168.1.1"))
}

func TestPathInit(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			AllowIPs: []string{"192.168.1.0/24"},
		},
		Headers: []*Header{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
		Queries: []*Query{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
	}

	path.Init(nil)

	assert.NotNil(path.ipFilter)
	assert.NotNil(path.ipFilterChain)
	assert.Equal(len(path.ipFilterChain.Filters()), 1)
	assert.Equal(path.method, httpprot.MALL)
	assert.NotNil(path.Headers[0].re)
	assert.NotNil(path.Queries[0].re)


	path.Methods = []string{"GET", "POST"}
	path.Init(nil)
	assert.True(path.method&httpprot.MGET != 0)
	assert.True(path.method&httpprot.MPOST != 0)
	assert.True(path.method&httpprot.MDELETE == 0)
}

func TestPathAllowIP(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs: []string{"192.168.1.0/24"},
		},
		Headers: []*Header{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
		Queries: []*Query{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
	}

	path.Init(nil)

	assert.True(path.AllowIP("192.168.1.1"))
	assert.False(path.AllowIP("10.168.1.1"))

	path.ipFilter = nil
	assert.True(path.AllowIP("192.168.1.1"))
	assert.True(path.AllowIP("10.168.1.1"))
}

func TestPathAllowIPChain(t *testing.T) {
}

func TestPathMatch(t *testing.T) {
}

func TestHeadersInit(t *testing.T) {
}

func TestHeadersMatch(t *testing.T) {
}

func TestQueriesInit(t *testing.T) {
}

func TestQueriesMatch(t *testing.T) {
}
