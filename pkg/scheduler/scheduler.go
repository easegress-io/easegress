package scheduler

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/registry"
	"gopkg.in/yaml.v2"
)

const (
	// To sync status, and rewatch configChan if need.
	checkStatusInterval = 5 * time.Second
	checkWatchInterval  = 3 * time.Second
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

		first     bool
		firstDone chan struct{}

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
		first:         true,
		firstDone:     make(chan struct{}),
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
		case <-time.After(checkStatusInterval):
			s.handleSyncStatus()
		case <-time.After(checkWatchInterval):
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

func (s Scheduler) keyToName(k string) string {
	return strings.TrimPrefix(k, s.cls.Layout().ConfigObjectPrefix())
}

func (s *Scheduler) handleKvs(kvs map[string]*string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("handleKvs() failed, err: %v", err)
		}
	}()

	if s.first {
		defer func() {
			s.first = false
			close(s.firstDone)
		}()
	}

	specs := make([]registry.Spec, 0)
	for k, v := range kvs {
		name := s.keyToName(k)
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
			logger.Errorf("BUG: inconsistent name in key and spec: %s, %s",
				name, spec.GetName())
			continue
		}

		specs = append(specs, spec)
	}

	sort.Sort(specsInOrder(specs))
	for _, spec := range specs {
		name := spec.GetName()
		unit, exists := s.units[name]
		if exists {
			unit.reload(spec)
			continue
		}

		newFunc, ok := unitNewFuncs[spec.GetKind()]
		if !ok {
			logger.Errorf("BUG: unsupported kind: %s", spec.GetKind())
			continue
		}

		newUnit, err := newFunc(spec, s.handlers, s.first /*first*/)
		if err != nil {
			logger.Errorf("BUG: %v", err)
			continue
		}
		s.units[name] = newUnit
	}
}

func (s *Scheduler) handleSyncStatus() {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("handleSyncStatus() failed, err: %v", err)
		}
	}()

	timestamp := uint64(time.Now().Unix())
	kvs := make(map[string]*string)
	for name, unit := range s.units {
		status := unit.status()
		status.InjectTimestamp(timestamp)

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

// FirstDone returns the firstDone channel,
// which will be closed after creating all objects at first time.
func (s *Scheduler) FirstDone() chan struct{} {
	return s.firstDone
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
