package informer

import (
	"fmt"
	"sync"

	yamljsontool "github.com/ghodss/yaml"
	"github.com/tidwall/gjson"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
)

const (
	// EventUpdate is the update infrom event.
	EventUpdate = "Update"
	// EventDelete is the delete infrom event.
	EventDelete = "Delete"

	// ScopeService is the scope of the whole service.
	ScopeService GJSONInformScope = ""

	// ScopeServiceObservability  is the scope of service observability.
	ScopeServiceObservability GJSONInformScope = "observability"

	// ScopeServiceResilience is the scope of service resilience.
	ScopeServiceResilience GJSONInformScope = "resilience"

	// GJSONScopeServiceCanary is the scope of service canary.
	ScopeServiceCanary GJSONInformScope = "canary"

	// ScopeServiceLoadBalance is the scope of service loadbalance.
	ScopeServiceLoadBalance GJSONInformScope = "loadBalance"

	// ScopeServiceLoadBalance is the scope of service loadbalance.
	ScopeServiceCircuitBreaker GJSONInformScope = "resilience.circuitBreaker"
)

type (
	// InformEvent is the type of inform event.
	InformEvent string

	// GJSONInformScope is the type of inform scope, in gjson syntax.
	GJSONInformScope string

	// ScopeFunc is the type of scope changed function.
	// value is empty when event is DeleteEvent. It's the complete service
	// structure in YAML format.
	ScopeFunc func(event InformEvent, value string)

	// PrefixFunc is the type of storage prefix get results changed function.
	// value is the updated result for the same prefix in storage
	PrefixFunc func(value map[string]string)

	// Informer is the interface for informing two type of storage changed.
	//  1. Based on one record's gjson field compared
	//  2. Based on same prefix's records changed
	Informer interface {
		OnScope(name string, gjsonScope GJSONInformScope, fn ScopeFunc) error
		OnPrefix(prefix string, fn PrefixFunc) error
	}

	ServiceInformer struct {
		store storage.Storage
		dict  map[string]bool

		mutex sync.Mutex
	}
)

var (
	// ErrAlreadyWatched is the error when calling informer to watch the same name/prefix
	ErrAlreadyWatched = fmt.Errorf("already watched")
)

// NewServiceInformer creates an Informer to watch service event.
func NewServiceInformer(store storage.Storage) Informer {
	return &ServiceInformer{
		store: store,
		dict:  make(map[string]bool),
		mutex:   sync.Mutex{},
	}
}

func (inf *ServiceInformer) compare(scope GJSONInformScope, origin, new string) bool {
	originSvc, err := yamljsontool.YAMLToJSON([]byte(origin))
	if err != nil {
		logger.Errorf("BUG, service informer using yamljsontool YAMLtoJSON faile, val:%s, err:v", origin, err)
		return true
	}

	newSvc, err := yamljsontool.YAMLToJSON([]byte(new))
	if err != nil {
		logger.Errorf("BUG, service informer using yamljsontool YAMLtoJSON faile, val:%s, err:v", new, err)
		return true
	}

	// compare the whole json string
	if scope == ScopeService {
		return string(originSvc) == string(newSvc)
	}

	// using gjson to extract desired field
	if gjson.Get(string(originSvc), string(scope)) == gjson.Get(string(newSvc), string(scope)) {
		return true
	}

	return false
}

func (inf *ServiceInformer) watch(ch <-chan *string, svcYAML string, scope GJSONInformScope, fn ScopeFunc) {
	for {
		select {
		case newSvcYAML := <-ch:
			if newSvcYAML == nil {
				fn(EventDelete, "")
				// after handleing delete event, the whole task should end
				return
			} else {
				if !inf.compare(scope, svcYAML, *newSvcYAML) {
					svcYAML = *newSvcYAML
					fn(EventUpdate, svcYAML)
				}
			}
		default:
		}
	}
}

func (inf *ServiceInformer) watchPrefix(ch <-chan map[string]*string, insList map[string]string, fn PrefixFunc) {
	for {
		select {
		case changeMap := <-ch:
			// only one key in changeMap
			for k, v := range changeMap {
				// delete
				if v == nil {
					if _, ok := insList[k]; ok {
						delete(insList, k)
						fn(insList)
					} // no need to callback
				} else {
					// add or update
					insList[k] = *v
					fn(insList)
				}
			}
		default:
		}
	}
}

// OnScope will call back fn when given scope of field change in serivce structure.
// Scope select support gjson syntax.
func (inf *ServiceInformer) OnScope(name string, gjsonScope GJSONInformScope, fn ScopeFunc) error {
	key := fmt.Sprintf("%s-%s", name, gjsonScope)
	inf.mutex.Lock()
	defer inf.mutex.Unlock()
	if _, ok := inf.dict[key]; ok {
		logger.Infof("already watching service:%s, scope:%s", name, gjsonScope)
		return ErrAlreadyWatched
	}

	var (
		s   *string
		err error
	)
	if s, err = inf.store.Get(layout.ServiceSpecKey(name)); err != nil {
		return err
	}
	if len(*s) == 0 {
		return spec.ErrServiceNotFound
	}

	watcher, err := inf.store.Watcher()
	if err != nil {
		return err
	}

	ch, err := watcher.Watch(layout.ServiceSpecKey(name))
	if err != nil {
		return err
	}

	inf.dict[key] = true
	go inf.watch(ch, *s, gjsonScope, fn)

	return nil
}

// OnPrefix will call back fn when given name service's instance list changed
func (inf *ServiceInformer) OnPrefix(name string, fn PrefixFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()
	prefix := layout.ServiceInstanceSpecPrefix(name)
	key := fmt.Sprintf("prefix-%s", prefix)
	if _, ok := inf.dict[key]; ok {
		logger.Infof("already wathed service:%s, prefix:%s", name, prefix)
		return ErrAlreadyWatched
	}

	watcher, err := inf.store.Watcher()
	if err != nil {
		return err
	}

	ch, err := watcher.WatchPrefix(prefix)
	if err != nil {
		return err
	}

	var insList map[string]string = make(map[string]string)
	if insList, err = inf.store.GetPrefix(prefix); err != nil {
		return err
	}

	inf.dict[key] = true
	go inf.watchPrefix(ch, insList, fn)

	return nil
}
