package scheduler

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httpproxy"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/registry"

	yaml "gopkg.in/yaml.v2"
)

const (
	// To sync status, and rewatch configChan if need.
	pollingTimeout = 5 * time.Second
)

type (
	serverUnit struct {
		server  *httpserver.HTTPServer
		runtime *httpserver.Runtime
	}

	proxyUnit struct {
		proxy   *httpproxy.HTTPProxy
		runtime *httpproxy.Runtime
	}

	// Scheduler is the brain to schedule all http objects.
	Scheduler struct {
		cls        cluster.Cluster
		configChan <-chan map[string]*string

		serverUnits map[string]*serverUnit
		proxyUnits  map[string]*proxyUnit
		handlers    *sync.Map // map[string]httpserver.Handler

		done chan struct{}
	}
)

func newServerUnit(serverSpec *httpserver.Spec, handlers *sync.Map) *serverUnit {
	runtime := httpserver.NewRuntime(handlers)
	return &serverUnit{
		server:  httpserver.New(serverSpec, runtime),
		runtime: runtime,
	}
}

func (su *serverUnit) update(serverSpec *httpserver.Spec) {
	su.server.Close()
	su.server = httpserver.New(serverSpec, su.runtime)
}

func (su *serverUnit) close() {
	su.server.Close()
	su.runtime.Close()
}

func newProxyUnit(proxySpec *httpproxy.Spec) *proxyUnit {
	runtime := httpproxy.NewRuntime()
	return &proxyUnit{
		proxy:   httpproxy.New(proxySpec, runtime),
		runtime: runtime,
	}
}

func (pu *proxyUnit) update(proxySpec *httpproxy.Spec) {
	pu.proxy.Close()
	pu.proxy = httpproxy.New(proxySpec, pu.runtime)
}

func (pu *proxyUnit) close() {
	pu.proxy.Close()
	pu.runtime.Close()
}

// New creates a Scheduler.
func New(cls cluster.Cluster) (*Scheduler, error) {
	s := &Scheduler{
		cls:         cls,
		serverUnits: make(map[string]*serverUnit),
		proxyUnits:  make(map[string]*proxyUnit),
		handlers:    &sync.Map{},
		done:        make(chan struct{}),
	}

	configChan, err := s.cls.WatchPrefix(cluster.ConfigObjectPrefix)
	if err != nil {
		return nil, fmt.Errorf("watch config objects failed: %v", err)
	}
	s.configChan = configChan

	go s.schedule()

	return s, nil
}

func (s *Scheduler) schedule() {
	for {
		select {
		case <-s.done:
			s.close()
			return
		case <-time.After(pollingTimeout):
			s.handleSyncStatus()
			s.handleRewatchIfNeed()
		case kvs, ok := <-s.configChan:
			if ok {
				s.handleKvs(kvs)
			} else {
				s.handleWatchFailed()
			}
		}
	}
}

func (s *Scheduler) handleRewatchIfNeed() {
	if s.configChan == nil {
		configChan, err := s.cls.WatchPrefix(cluster.ConfigObjectPrefix)
		if err != nil {
			logger.Errorf("watch config objects failed: %v", err)
		} else {
			s.configChan = configChan
		}
	}
}

func (s *Scheduler) handleWatchFailed() {
	s.configChan = nil
}

func (s *Scheduler) handleKvs(kvs map[string]*string) {
	for k, v := range kvs {
		name := strings.TrimPrefix(k, cluster.ConfigObjectPrefix)
		if v == nil {
			s.handleDelete(name)
			continue
		}

		spec, err := registry.SpecFromYAML(*v)
		if err != nil {
			logger.Errorf("BUG: bad spec(err: %s): %v", err, *v)
			continue
		}

		if name != spec.GetName() {
			logger.Errorf("BUG: inconsistent name in path and spec: %s, %s",
				name, spec.GetName())
		}

		switch spec.GetKind() {
		case httpserver.Kind:
			s.reloadHTTPServer(spec)
		case httpproxy.Kind:
			s.reloadHTTPProxy(spec)
		}
	}
}

func (s *Scheduler) handleDelete(name string) {
	if su, exists := s.serverUnits[name]; exists {
		su.close()
		delete(s.serverUnits, name)
		return
	}

	if pu, exists := s.proxyUnits[name]; exists {
		pu.close()
		delete(s.proxyUnits, name)
		return
	}
}

func (s *Scheduler) reloadHTTPServer(spec registry.Spec) {
	serverSpec, ok := spec.(*httpserver.Spec)
	if !ok {
		logger.Errorf("BUG: spec want *httpserver.Spec got %T", spec)
		return
	}

	name := spec.GetName()

	su := s.serverUnits[name]
	if su == nil {
		su = newServerUnit(serverSpec, s.handlers)
		s.serverUnits[name] = su
	} else {
		su.update(serverSpec)
	}
}

func (s *Scheduler) reloadHTTPProxy(spec registry.Spec) {
	proxySpec, ok := spec.(*httpproxy.Spec)
	if !ok {
		logger.Errorf("BUG: spec want *httpproxy.Spec got %T", spec)
		return
	}

	name := spec.GetName()

	pu := s.proxyUnits[name]
	if pu == nil {
		pu = newProxyUnit(proxySpec)
		s.proxyUnits[name] = pu
	} else {
		pu.update(proxySpec)
	}
	s.handlers.Store(name, pu.proxy)
}

func (s *Scheduler) handleSyncStatus() {
	kvs := make(map[string]*string)
	for name, su := range s.serverUnits {
		status := su.runtime.Status()
		buff, err := yaml.Marshal(status)
		if err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v",
				status, err)
			continue
		}
		key := fmt.Sprintf(cluster.StatusObjectFormat, name)
		value := string(buff)
		kvs[key] = &value
	}
	for name, pu := range s.proxyUnits {
		status := pu.runtime.Status()
		buff, err := yaml.Marshal(status)
		if err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v",
				status, err)
			continue
		}
		key := fmt.Sprintf(cluster.StatusObjectFormat, name)
		value := string(buff)
		kvs[key] = &value
	}

	err := s.cls.PutAndDeleteUnderLease(kvs)
	if err != nil {
		logger.Errorf("sync runtime failed: %v", err)
	}
}

// Close closes Scheduler.
func (s *Scheduler) Close(wg *sync.WaitGroup) {
	defer wg.Done()
	s.done <- struct{}{}
	<-s.done
}

func (s *Scheduler) close() {
	for _, su := range s.serverUnits {
		su.close()
	}

	for _, pu := range s.proxyUnits {
		pu.close()
	}

	close(s.done)
}
