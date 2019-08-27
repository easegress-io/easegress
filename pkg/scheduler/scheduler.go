package scheduler

import (
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"

	"gopkg.in/yaml.v2"
)

const (
	pollTimouet = 5 * time.Second
)

type (
	// Scheduler is the brain to schedule all http objects.
	Scheduler struct {
		cls        cluster.Cluster
		prefix     string
		configChan <-chan map[string]*string

		redeleteStatus map[string]struct{}
		runningObjects map[string]*runningObject
		handlers       *sync.Map // map[string]httpserver.Handler

		first     bool
		firstDone chan struct{}

		done chan struct{}
	}

	runningObject struct {
		object Object
		spec   Spec
		or     *ObjectRecord
	}

	runningPluginsByDepend []*runningObject
)

func (r runningPluginsByDepend) Len() int      { return len(r) }
func (r runningPluginsByDepend) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r runningPluginsByDepend) Less(i, j int) bool {
	iKind, iDependKinds := r[i].or.Kind, r[i].or.DependObjectKinds
	jKind, jDependKinds := r[j].or.Kind, r[j].or.DependObjectKinds

	for _, jDependKind := range jDependKinds {
		if iKind == jDependKind {
			return true
		}
	}

	for _, iDependKind := range iDependKinds {
		if jKind == iDependKind {
			return false
		}
	}

	return iKind < jKind
}

// MustNew creates a Scheduler.
func MustNew(cls cluster.Cluster) *Scheduler {
	s := &Scheduler{
		cls:            cls,
		prefix:         cls.Layout().ConfigObjectPrefix(),
		redeleteStatus: make(map[string]struct{}),
		runningObjects: make(map[string]*runningObject),
		handlers:       &sync.Map{},
		first:          true,
		firstDone:      make(chan struct{}),
		done:           make(chan struct{}),
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
		case <-time.After(pollTimouet):
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

func (s Scheduler) keyToName(k string) string {
	return strings.TrimPrefix(k, s.cls.Layout().ConfigObjectPrefix())
}

func (s *Scheduler) handleKvs(kvs map[string]*string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("recover from handleKvs, err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	if s.first {
		defer func() {
			s.first = false
			close(s.firstDone)
		}()
	}

	runningObjects := make([]*runningObject, 0)
	for k, v := range kvs {
		name := s.keyToName(k)
		if v == nil {
			if runningObject, exists := s.runningObjects[name]; exists {
				runningObject.object.Close()
				delete(s.runningObjects, name)
				err := s.cls.Delete(s.cls.Layout().StatusObjectKey(name))
				// NOTE: The Delete operation might not succeed,
				// but we can't redo it here infinitely.
				if err != nil {
					s.redeleteStatus[name] = struct{}{}
				}
			}
			continue
		}

		spec, err := SpecFromYAML(*v)
		if err != nil {
			logger.Errorf("BUG: spec from yaml failed: %s: %v", *v, err)
			continue
		}
		if name != spec.GetName() {
			logger.Errorf("BUG: inconsistent name in key and spec: %s, %s",
				name, spec.GetName())
			continue
		}
		or, exists := objectBook[spec.GetKind()]
		if !exists {
			logger.Errorf("BUG: unsupported kind: %s", spec.GetKind())
			continue
		}

		runningObjects = append(runningObjects, &runningObject{
			spec: spec,
			or:   or,
		})
	}

	sort.Sort(runningPluginsByDepend(runningObjects))

	for _, runningObject := range runningObjects {
		name := runningObject.spec.GetName()

		var prevValue reflect.Value
		prevObject := s.runningObjects[name]
		if prevObject == nil {
			prevValue = reflect.New(runningObject.or.objectType).Elem()
		} else {
			prevValue = reflect.ValueOf(prevObject.object)
		}
		in := []reflect.Value{
			reflect.ValueOf(runningObject.spec),
			prevValue,
			reflect.ValueOf(s.handlers),
		}
		runningObject.object = reflect.ValueOf(runningObject.or.NewFunc).
			Call(in)[0].Interface().(Object)
		s.runningObjects[name] = runningObject
	}
}

func (s *Scheduler) handleSyncStatus() {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("recover from handleSyncStatus, err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	timestamp := uint64(time.Now().Unix())
	kvs := make(map[string]*string)
	for name, runningObject := range s.runningObjects {
		status := reflect.ValueOf(runningObject.object).
			MethodByName("Status").Call(nil)[0].Interface()
		reflect.ValueOf(status).Elem().FieldByName("Timestamp").SetUint(timestamp)

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

	for name := range s.redeleteStatus {
		if _, exists := s.runningObjects[name]; exists {
			continue
		}
		kvs[name] = nil
	}

	err := s.cls.PutAndDeleteUnderLease(kvs)
	if err != nil {
		logger.Errorf("sync runtime failed: put status failed: %v", err)
	} else {
		s.redeleteStatus = make(map[string]struct{})
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
	for _, runningObject := range s.runningObjects {
		runningObject.object.Close()
	}

	close(s.done)
}
