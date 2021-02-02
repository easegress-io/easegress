package supervisor

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"sync"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/util/timetool"

	yaml "gopkg.in/yaml.v2"
)

const (
	maxStatusesRecordCount = 10
)

var (
	// Global is the globsl singleton supervisor.
	Global *Supervisor
)

// InitGlobalSupervisor initialize the single instance of Supervisor at runtime.
// This function must be invoked in the main() immediately after the Supervisor instance created.
// The reason not to use a singleton mode is to be unit test friendly.
//
func InitGlobalSupervisor(sdl *Supervisor) {
	if Global != nil {
		panic("global supervisor has been initialized")
	}

	Global = sdl
}

type (
	// Supervisor is the brain to schedule all http objects.
	Supervisor struct {
		storage *storage
		timer   *timetool.DistributedTimer

		runningObjects map[string]*runningObject
		handlers       *sync.Map // map[string]httpserver.Handler

		// sorted by timestamp in ascending order
		statusesRecords      []*StatusesRecord
		StatusesRecordsMutex sync.RWMutex

		first     bool
		firstDone chan struct{}

		done chan struct{}
	}

	// StatusesRecord is the history record for status of every running object.
	StatusesRecord struct {
		Statuses     map[string]interface{}
		UnixTimestmp int64
	}

	runningObject struct {
		object Object
		config string
		spec   Spec
		or     *ObjectRecord
	}

	runningFiltersByDepend []*runningObject
)

func (r runningFiltersByDepend) Len() int      { return len(r) }
func (r runningFiltersByDepend) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r runningFiltersByDepend) Less(i, j int) bool {
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

// MustNew creates a Supervisor.
func MustNew(opt *option.Options, cls cluster.Cluster) *Supervisor {
	s := &Supervisor{
		storage:        newStorage(opt, cls),
		timer:          timetool.NewDistributedTimer(nextSyncStatusDuration),
		runningObjects: make(map[string]*runningObject),
		handlers:       &sync.Map{},
		first:          true,
		firstDone:      make(chan struct{}),
		done:           make(chan struct{}),
	}

	go s.schedule()

	return s
}

// SendHTTPRequet sent the http context to another object which must be an HTTPHandler.
func (s *Supervisor) SendHTTPRequet(targetHTTPHandler string, httpContext context.HTTPContext) (err error) {
	handler, ok := s.handlers.Load(targetHTTPHandler)
	if !ok {
		return fmt.Errorf("target http object or pipeline not found: %s", targetHTTPHandler)

	}

	handler.(HTTPHandler).Handle(httpContext)
	return nil
}

func (s *Supervisor) schedule() {
	for {
		select {
		case <-s.done:
			s.close()
			return
		case t := <-s.timer.C:
			s.syncStatus(t.Unix())
		case config := <-s.storage.watchConfig():
			s.handleConfig(config)
		}
	}
}

func (s *Supervisor) diffConfig(config map[string]string) (
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

func (s *Supervisor) handleConfig(config map[string]string) {
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
		func() {
			defer func() {
				if err := recover(); err != nil {
					logger.Errorf("recover from handleKvs when creating or updating cfg of %s, err: %v, stack trace:\n%s\n",
						name, err, debug.Stack())
				}
			}()

			spec, err := SpecFromYAML(v)
			if err != nil {
				logger.Errorf("BUG: spec from yaml failed: %s: %v", v, err)
				return
			}

			if name != spec.GetName() {
				logger.Errorf("BUG: inconsistent name in key and spec: %s, %s",
					name, spec.GetName())
				return
			}
			or, exists := objectBook[spec.GetKind()]
			if !exists {
				logger.Errorf("BUG: unsupported kind: %s", spec.GetKind())
				return
			}

			runningObjects = append(runningObjects, &runningObject{
				config: v,
				spec:   spec,
				or:     or,
			})
		}()
	}

	sort.Sort(runningFiltersByDepend(runningObjects))

	for _, runningObject := range runningObjects {
		name := runningObject.spec.GetName()
		func() {
			defer func() {
				if err := recover(); err != nil {
					logger.Errorf("recover from handleKvs when updating running obj %s,  err: %v, stack trace:\n%s\n",
						name, err, debug.Stack())
				}
			}()

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
		}()
	}
}

func (s *Supervisor) syncStatus(unixTimestamp int64) {
	statuses := make(map[string]string)
	statusesRecord := &StatusesRecord{
		Statuses:     map[string]interface{}{},
		UnixTimestmp: unixTimestamp,
	}
	for name, runningObject := range s.runningObjects {
		func() {
			defer func() {
				if err := recover(); err != nil {
					logger.Errorf("recover from handleSyncStatus, err: %v, stack trace:\n%s\n",
						err, debug.Stack())
				}
			}()

			status := reflect.ValueOf(runningObject.object).
				MethodByName("Status").Call(nil)[0].Interface()
			reflect.ValueOf(status).Elem().FieldByName("Timestamp").SetInt(unixTimestamp)

			statusesRecord.Statuses[name] = status

			buff, err := yaml.Marshal(status)
			if err != nil {
				logger.Errorf("BUG: marshal %#v to yaml failed: %v",
					status, err)
				return
			}
			statuses[name] = string(buff)
		}()
	}

	s.addStatusesRecord(statusesRecord)

	s.storage.syncStatus(statuses)
}

func (s *Supervisor) addStatusesRecord(statusesRecord *StatusesRecord) {
	s.StatusesRecordsMutex.Lock()
	defer s.StatusesRecordsMutex.Unlock()

	s.statusesRecords = append(s.statusesRecords, statusesRecord)
	if len(s.statusesRecords) > maxStatusesRecordCount {
		s.statusesRecords = s.statusesRecords[1:]
	}
}

// GetStatusesRecords return the latest statuses records.
func (s *Supervisor) GetStatusesRecords() []*StatusesRecord {
	s.StatusesRecordsMutex.RLock()
	defer s.StatusesRecordsMutex.RUnlock()

	records := make([]*StatusesRecord, len(s.statusesRecords))
	for i, record := range s.statusesRecords {
		records[i] = record
	}

	return records
}

// FirstDone returns the firstDone channel,
// which will be closed after creating all objects at first time.
func (s *Supervisor) FirstDone() chan struct{} {
	return s.firstDone
}

// Close closes Supervisor.
func (s *Supervisor) Close(wg *sync.WaitGroup) {
	defer wg.Done()
	s.done <- struct{}{}
	<-s.done
}

func (s *Supervisor) close() {
	s.storage.close()
	s.timer.Close()

	for _, runningObject := range s.runningObjects {
		runningObject.object.Close()
	}

	close(s.done)
}
