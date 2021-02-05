package supervisor

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/storage"
)

// Global is the global supervisor.
var Global *Supervisor

// InitGlobalSupervisor initializes global supervisor.
// FIXME: Prune Global stuff after sending super to filters.
func InitGlobalSupervisor(super *Supervisor) {
	Global = super
}

const (
	maxStatusesRecordCount = 10
)

type (
	// Supervisor manages all objects.
	Supervisor struct {
		options *option.Options
		cls     cluster.Cluster

		storage           *storage.Storage
		runningCategories map[ObjectCategory]*RunningCategory
		firstHandle       bool
		firstHandleDone   chan struct{}
		done              chan struct{}
	}

	// RunningCategory is the bucket to gather running objects in the same category.
	RunningCategory struct {
		mutex          sync.RWMutex
		category       ObjectCategory
		runningObjects map[string]*RunningObject
	}

	// RunningObject is the running object.
	RunningObject struct {
		object Object
		config string
		spec   ObjectSpec
	}
)

func newRunningObjectFromConfig(config string) (*RunningObject, error) {
	spec, err := SpecFromYAML(config)
	if err != nil {
		return nil, fmt.Errorf("spec from yaml failed: %s: %v", config, err)
	}

	return newRunningObjectFromSpec(spec)
}

func newRunningObjectFromSpec(spec ObjectSpec) (*RunningObject, error) {
	registerObject, exists := objectRegistry[spec.GetKind()]
	if !exists {
		return nil, fmt.Errorf("unsupported kind: %s", spec.GetKind())
	}

	obj := reflect.New(reflect.TypeOf(registerObject).Elem()).Interface()
	object, ok := obj.(Object)
	if !ok {
		return nil, fmt.Errorf("%T is not an object", obj)
	}

	return &RunningObject{
		config: YAMLFromSpec(spec),
		spec:   spec,
		object: object,
	}, nil
}

// Instance returns the instance of the object.
func (ro *RunningObject) Instance() Object {
	return ro.object
}

// Spec returns the spec of the object.
func (ro *RunningObject) Spec() ObjectSpec {
	return ro.spec
}

// Config returns the raw config of the object.
func (ro *RunningObject) Config() string {
	return ro.config
}

func (ro *RunningObject) renewWithRecovery(prevInstance Object, super *Supervisor) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from renew, err: %v, stack trace:\n%s\n",
				ro.spec.GetName(), err, debug.Stack())
		}
	}()
	ro.Instance().Renew(ro.Spec(), prevInstance, super)
}

func (ro *RunningObject) closeWithRecovery() {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from close, err: %v, stack trace:\n%s\n",
				ro.spec.GetName(), err, debug.Stack())
		}
	}()

	ro.object.Close()
}

// MustNew creates a Supervisor.
func MustNew(opt *option.Options, cls cluster.Cluster) *Supervisor {
	s := &Supervisor{
		options: opt,
		cls:     cls,

		storage:           storage.New(opt, cls),
		runningCategories: make(map[ObjectCategory]*RunningCategory),
		firstHandle:       true,
		firstHandleDone:   make(chan struct{}),
		done:              make(chan struct{}),
	}

	for _, category := range objectOrderedCategories {
		s.runningCategories[category] = &RunningCategory{
			category:       category,
			runningObjects: make(map[string]*RunningObject),
		}
	}

	s.initSystemControllers()

	go s.run()

	return s
}

// Options returns the options applied to supervisor.
func (s *Supervisor) Options() *option.Options {
	return s.options
}

// Cluster return the cluster applied to supervisor.
func (s *Supervisor) Cluster() cluster.Cluster {
	return s.cls
}

func (s *Supervisor) initSystemControllers() {
	for kind, rootObject := range objectRegistry {
		if rootObject.Category() != CategorySystemController {
			continue
		}

		spec := rootObject.DefaultSpec()
		ro, err := newRunningObjectFromSpec(spec)
		if err != nil {
			panic(err)
		}

		ro.renewWithRecovery(nil, s)
		s.runningCategories[CategorySystemController].runningObjects[kind] = ro

		logger.Infof("create %s", spec.GetName())
	}
}

func (s *Supervisor) run() {
	for {
		select {
		case <-s.done:
			s.close()
			return
		case config := <-s.storage.WatchConfig():
			s.applyConfig(config)
		}
	}
}

func (s *Supervisor) applyConfig(config map[string]string) {
	if s.firstHandle {
		defer func() {
			s.firstHandle = false
			close(s.firstHandleDone)
		}()
	}

	// Create, update, delete from high to low priority.
	for _, category := range objectOrderedCategories {
		// NOTE: System controller can't be manipulated after initialized.
		if category != CategorySystemController {
			s.applyConfigInCategory(config, category)
		}
	}
}

func (s *Supervisor) applyConfigInCategory(config map[string]string, category ObjectCategory) {
	rc := s.runningCategories[category]

	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	// Delete running object.
	for name, ro := range rc.runningObjects {
		if _, exists := config[name]; !exists {
			ro.closeWithRecovery()
			logger.Errorf("delete %s", name)
		}
	}

	// Create or update running object.
	for name, yamlConfig := range config {
		var prevInstance Object
		prev, exists := rc.runningObjects[name]
		if exists {
			// No need to update if the config not changed.
			if yamlConfig == prev.Config() {
				continue
			}
			prevInstance = prev.Instance()
		}

		ro, err := newRunningObjectFromConfig(yamlConfig)
		if err != nil {
			logger.Errorf("BUG: %s: %v", name, err)
			continue
		}
		if ro.object.Category() != category {
			continue
		}
		if name != ro.Spec().GetName() {
			logger.Errorf("BUG: key and spec got different names: %s vs %s",
				name, ro.Spec().GetName())
			continue
		}

		ro.renewWithRecovery(prevInstance, s)

		rc.runningObjects[name] = ro

		if prevInstance == nil {
			logger.Infof("create %s", name)
		} else {
			logger.Infof("update %s", name)
		}
	}
}

// WalkFunc is the type of the function called for
// each running object visited by WalkRunningObjects.
type WalkFunc func(runningObject *RunningObject) bool

// WalkRunningObjects walks every running object until walkFn returns false.
func (s *Supervisor) WalkRunningObjects(walkFn WalkFunc, category ObjectCategory) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkRunningObjects recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}

	}()

	for _, rc := range s.runningCategories {
		if category != CategoryAll && category != rc.category {
			continue
		}

		func() {
			rc.mutex.RLock()
			defer rc.mutex.RUnlock()

			for _, runningObject := range rc.runningObjects {
				toContinue := walkFn(runningObject)
				if !toContinue {
					break
				}
			}
		}()
	}
}

// GetRunningObject returns the running object with the existing flag.
// If the category is empty string, GetRunningObject will try to find
// the running object in every category.
func (s *Supervisor) GetRunningObject(name string, category ObjectCategory) (ro *RunningObject, exists bool) {
	searchFromCategory := func(rc *RunningCategory) {
		rc.mutex.RLock()
		defer rc.mutex.RUnlock()

		ro, exists = rc.runningObjects[name]
	}

	switch category {
	case CategoryAll:
		for _, rc := range s.runningCategories {
			searchFromCategory(rc)
			if exists {
				return
			}
		}
	default:
		rc, rcExists := s.runningCategories[category]
		if !rcExists {
			logger.Errorf("BUG: category %s not found", category)
			return nil, false
		}
		searchFromCategory(rc)
	}

	return
}

// FirstHandleDone returns the firstHandleDone channel,
// which will be closed after creating all objects at first time.
func (s *Supervisor) FirstHandleDone() chan struct{} {
	return s.firstHandleDone
}

// Close closes Supervisor.
func (s *Supervisor) Close(wg *sync.WaitGroup) {
	defer wg.Done()
	s.done <- struct{}{}
	<-s.done
}

func (s *Supervisor) close() {
	s.storage.Close()

	// Close from low to high priority.
	for i := len(objectOrderedCategories) - 1; i >= 0; i-- {
		rc := s.runningCategories[objectOrderedCategories[i]]
		rc.mutex.Lock()
		for _, ro := range rc.runningObjects {
			ro.closeWithRecovery()
			logger.Infof("delete %s", ro.spec.GetName())
		}
		rc.mutex.Unlock()
	}

	close(s.done)
}
