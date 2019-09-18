package scheduler

import (
	"reflect"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"

	yaml "gopkg.in/yaml.v2"
)

const (
	syncStatusInterval = 5 * time.Second
)

type (
	// Scheduler is the brain to schedule all http objects.
	Scheduler struct {
		storage *storage

		runningObjects map[string]*runningObject
		handlers       *sync.Map // map[string]httpserver.Handler

		first     bool
		firstDone chan struct{}

		done chan struct{}
	}

	runningObject struct {
		object Object
		config string
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
func MustNew(opt *option.Options, cls cluster.Cluster) *Scheduler {
	s := &Scheduler{
		storage:        newStorage(opt, cls),
		runningObjects: make(map[string]*runningObject),
		handlers:       &sync.Map{},
		first:          true,
		firstDone:      make(chan struct{}),
		done:           make(chan struct{}),
	}

	go s.schedule()

	return s
}

func (s *Scheduler) schedule() {
	for {
		select {
		case <-s.done:
			s.close()
			return
		case <-time.After(syncStatusInterval):
			s.syncStatus()
		case config := <-s.storage.watchConfig():
			s.handleConfig(config)
		}
	}
}

func (s *Scheduler) diffConfig(config map[string]string) (
	createOrUpdate, del map[string]string) {

	createOrUpdate, del = make(map[string]string), make(map[string]string)

	for k, v := range config {
		runningObject, exist := s.runningObjects[k]
		if !exist {
			createOrUpdate[k] = v
		} else if v != runningObject.config {
			createOrUpdate[k] = v
		}
	}

	for name := range s.runningObjects {
		if _, exists := config[name]; !exists {
			del[name] = ""
		}
	}

	return
}

func (s *Scheduler) handleConfig(config map[string]string) {
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

	createOrupdate, del := s.diffConfig(config)

	for name := range del {
		runningObject, exists := s.runningObjects[name]
		if !exists {
			logger.Errorf("BUG: delete %s not found", name)
			continue
		}
		runningObject.object.Close()
		delete(s.runningObjects, name)
	}

	runningObjects := make([]*runningObject, 0)
	for name, v := range createOrupdate {
		spec, err := SpecFromYAML(v)
		if err != nil {
			logger.Errorf("BUG: spec from yaml failed: %s: %v", v, err)
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
			config: v,
			spec:   spec,
			or:     or,
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

func (s *Scheduler) syncStatus() {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("recover from handleSyncStatus, err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	timestamp := time.Now().Unix()
	statuses := make(map[string]string)
	for name, runningObject := range s.runningObjects {
		status := reflect.ValueOf(runningObject.object).
			MethodByName("Status").Call(nil)[0].Interface()
		reflect.ValueOf(status).Elem().FieldByName("Timestamp").SetInt(timestamp)

		buff, err := yaml.Marshal(status)
		if err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v",
				status, err)
			continue
		}
		statuses[name] = string(buff)
	}

	s.storage.syncStatus(statuses)
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
	s.storage.close()

	for _, runningObject := range s.runningObjects {
		runningObject.object.Close()
	}

	close(s.done)
}
