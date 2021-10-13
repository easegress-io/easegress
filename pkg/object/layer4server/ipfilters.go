package layer4server

import (
	"reflect"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (
	ipFilters struct {
		rules atomic.Value
	}

	ipFiltersRules struct {
		spec     *ipfilter.Spec
		ipFilter *ipfilter.IPFilter
	}
)

func newIpFilters(spec *ipfilter.Spec) *ipFilters {
	m := &ipFilters{}

	m.rules.Store(&ipFiltersRules{
		spec:     spec,
		ipFilter: newIPFilter(spec),
	})
	return m
}

func (i *ipFilters) AllowIP(ip string) bool {
	rules := i.rules.Load().(*ipFiltersRules)
	if rules == nil {
		return true
	}
	return rules.ipFilter.Allow(ip)
}

func (i *ipFilters) reloadRules(spec *ipfilter.Spec) {
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

func (r *ipFiltersRules) pass(downstreamIp string) bool {
	if r.ipFilter == nil {
		return true
	}
	return r.ipFilter.Allow(downstreamIp)
}
