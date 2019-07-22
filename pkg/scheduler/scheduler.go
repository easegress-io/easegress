package scheduler

import (
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/registry"
	yaml "gopkg.in/yaml.v2"
)

const (
	// To sync status, and rewatch configChan if need.
	pollingTimeout = 5 * time.Second
)

type (
	// Scheduler is the brain to schedule all http objects.
	Scheduler struct {
		cls        cluster.Cluster
		prefix     string
		configChan <-chan map[string]*string

		redeleteUnits map[string]struct{}
		units         map[string]unit
		handlers      *sync.Map // map[string]httpserver.Handler

		done chan struct{}
	}
)

// MustNew creates a Scheduler.
func MustNew(cls cluster.Cluster) *Scheduler {
	s := &Scheduler{
		cls:           cls,
		prefix:        cls.Layout().ConfigObjectPrefix(),
		redeleteUnits: make(map[string]struct{}),
		units:         make(map[string]unit),
		handlers:      &sync.Map{},
		done:          make(chan struct{}),
	}

	configChan, err := s.cls.WatchPrefix(s.prefix)
	if err != nil {
		logger.Errorf("scheduler watch config objects failed: %v", err)
	}
	s.configChan = configChan

	go s.schedule()

	return s
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
				logger.Errorf("watch %s failed", s.prefix)
				s.handleWatchFailed()
			}
		}
	}
}

func (s *Scheduler) handleRewatchIfNeed() {
	if s.configChan == nil {
		configChan, err := s.cls.WatchPrefix(s.prefix)
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
	// TODO: In the every first time,
	// it should create all Seckills synchronically for loading massive redis data,
	// then HTTPProxy, HTTPServer.
	for k, v := range kvs {
		name := strings.TrimPrefix(k, s.cls.Layout().ConfigObjectPrefix())
		unit, exists := s.units[name]
		if v == nil {
			if exists {
				unit.close()
				delete(s.units, name)
				err := s.cls.Delete(s.cls.Layout().StatusObjectKey(name))
				// NOTE: The Delete operation might not succeed,
				// but we can't redo it here infinitely.
				if err != nil {
					s.redeleteUnits[name] = struct{}{}
				}
			}
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
			continue
		}

		if exists {
			unit.reload(spec)
			continue
		}

		newFunc, ok := unitNewFuncs[spec.GetKind()]
		if !ok {
			logger.Errorf("BUG: unsupported kind: %s", spec.GetKind())
			continue
		}

		newUnit, err := newFunc(spec, s.handlers)
		if err != nil {
			logger.Errorf("BUG: %v", err)
			continue
		}
		s.units[name] = newUnit
	}
}

func (s *Scheduler) handleSyncStatus() {
	kvs := make(map[string]*string)
	for name, unit := range s.units {
		status := unit.status()
		buff, err := yaml.Marshal(status)
		if err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v",
				status, err)
			continue
		}
		key := s.cls.Layout().StatusObjectKey(name)
		value := string(buff)
		kvs[key] = &value
	}

	for name := range s.redeleteUnits {
		if _, exists := s.units[name]; exists {
			continue
		}
		kvs[name] = nil
	}

	err := s.cls.PutAndDeleteUnderLease(kvs)
	if err != nil {
		logger.Errorf("sync runtime failed: %v", err)
	} else {
		s.redeleteUnits = make(map[string]struct{})
	}
}

// Close closes Scheduler.
func (s *Scheduler) Close(wg *sync.WaitGroup) {
	defer wg.Done()
	s.done <- struct{}{}
	<-s.done
}

func (s *Scheduler) close() {
	for _, unit := range s.units {
		unit.close()
	}

	close(s.done)
}
