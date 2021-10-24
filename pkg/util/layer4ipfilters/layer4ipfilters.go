package layer4ipfilters

import (
	"reflect"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (
	Layer4IpFilters struct {
		rules atomic.Value
	}

	ipFiltersRules struct {
		spec     *ipfilter.Spec
		ipFilter *ipfilter.IPFilter
	}
)

func NewLayer4IPFilters(spec *ipfilter.Spec) *Layer4IpFilters {
	m := &Layer4IpFilters{}

	m.rules.Store(&ipFiltersRules{
		spec:     spec,
		ipFilter: newIPFilter(spec),
	})
	return m
}

func (i *Layer4IpFilters) AllowIP(ip string) bool {
	rules := i.rules.Load().(*ipFiltersRules)
	if rules == nil || rules.spec == nil {
		return true
	}
	return rules.ipFilter.Allow(ip)
}

func (i *Layer4IpFilters) ReloadRules(spec *ipfilter.Spec) {
	if spec == nil {
		i.rules.Store(&ipFiltersRules{})
		return
	}

	old := i.rules.Load().(*ipFiltersRules)
	if reflect.DeepEqual(old.spec, spec) {
		return
	}

	rules := &ipFiltersRules{
		spec:     spec,
		ipFilter: newIPFilter(spec),
	}
	i.rules.Store(rules)
}

func newIPFilter(spec *ipfilter.Spec) *ipfilter.IPFilter {
	if spec == nil {
		return nil
	}
	return ipfilter.New(spec)
}

func (r *ipFiltersRules) pass(downstreamIP string) bool {
	if r.ipFilter == nil {
		return true
	}
	return r.ipFilter.Allow(downstreamIP)
}
