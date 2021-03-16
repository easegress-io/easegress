package informer

import (
	"fmt"
	"sync"

	yamljsontool "github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
)

const (
	// TODO: Support EventCreate.

	// EventUpdate is the update infrom event.
	EventUpdate = "Update"
	// EventDelete is the delete infrom event.
	EventDelete = "Delete"

	// AllParts is the path of the whole structure.
	AllParts GJSONPath = ""

	// ServiceObservability  is the path of service observability.
	ServiceObservability GJSONPath = "observability"

	// ServiceResilience is the path of service resilience.
	ServiceResilience GJSONPath = "resilience"

	// ServiceCanary is the path of service canary.
	ServiceCanary GJSONPath = "canary"

	// ServiceLoadBalance is the path of service loadbalance.
	ServiceLoadBalance GJSONPath = "loadBalance"

	// ServiceCircuitBreaker is the path of service resielience's circuritBreaker part.
	ServiceCircuitBreaker GJSONPath = "resilience.circuitBreaker"
)

type (
	// Event is the type of inform event.
	Event string

	// GJSONPath is the type of inform path, in gjson syntax.
	GJSONPath string

	specHandleFunc  func(event Event, value string) bool
	specsHandleFunc func(map[string]string) bool

	// The returning boolean flag of all callback functions means
	// if the stuff continues to be watched.

	// ServiceSpecFunc is the callback function type for service spec.
	ServiceSpecFunc func(event Event, serviceSpec *spec.Service) bool

	// ServiceSpecFunc is the callback function type for service specs.
	ServiceSpecsFunc func(value map[string]*spec.Service) bool

	// ServiceInstanceSpecFunc is the callback function type for service instance spec.
	ServicesInstanceSpecFunc func(event Event, instanceSpec *spec.ServiceInstanceSpec) bool

	// ServiceInstanceSpecFunc is the callback function type for service instance specs.
	ServiceInstanceSpecsFunc func(value map[string]*spec.ServiceInstanceSpec) bool

	// ServiceInstanceStatusFunc is the callback function type for service instance status.
	ServiceInstanceStatusFunc func(event Event, value *spec.ServiceInstanceStatus) bool

	// ServiceInstanceStatusesFunc is the callback function type for service instance statuses.
	ServiceInstanceStatusesFunc func(value map[string]*spec.ServiceInstanceStatus) bool

	// TenantSpecFunc is the callback function type for tenant spec.
	TenantSpecFunc func(event Event, value *spec.Tenant) bool

	// TenantSpecsFunc is the callback function type for tenant specs.
	TenantSpecsFunc func(value map[string]*spec.Tenant) bool

	// Informer is the interface for informing two type of storage changed for every Mesh spec structure.
	//  1. Based on comparison between old and new part of entry.
	//  2. Based on comparison on entries with the same prefix.
	Informer interface {
		OnPartOfServiceSpec(serviceName string, gjsonPath GJSONPath, fn ServiceSpecFunc) error
		OnSerivceSpecs(servicePrefix string, fn ServiceSpecsFunc) error

		OnPartOfInstanceSpec(serviceName, instanceID string, gjsonPath GJSONPath, fn ServicesInstanceSpecFunc) error
		OnServiceInstanceSpecs(serviceName string, fn ServiceInstanceSpecsFunc) error

		OnPartOfServiceInstanceStatus(serviceName, instanceID string, gjsonPath GJSONPath, fn ServiceInstanceStatusFunc) error
		OnServiceInstanceStatuses(serviceName string, fn ServiceInstanceStatusesFunc) error

		OnPartOfTenantSpec(tenantName string, gjsonPath GJSONPath, fn TenantSpecFunc) error
		OnTenantSpecs(tenantPrefix string, fn TenantSpecsFunc) error

		Close()
	}

	// meshInformer is the informer for mesh usage
	meshInformer struct {
		mutex    sync.Mutex
		store    storage.Storage
		watchers map[string]storage.Watcher

		closed bool
		done   chan struct{}
	}
)

var (
	// ErrAlreadyWatched is the error when watch the same entry again.
	ErrAlreadyWatched = fmt.Errorf("already watched")

	// ErrClosed is the error when watching a closed informer.
	ErrClosed = fmt.Errorf("informer already been closed")

	// ErrNotFound is the error when watching an entry which is not found.
	ErrNotFound = fmt.Errorf("not found")
)

// NewInformer creates an informer.
func NewInformer(store storage.Storage) Informer {
	inf := &meshInformer{
		store:    store,
		watchers: make(map[string]storage.Watcher),
		mutex:    sync.Mutex{},
		done:     make(chan struct{}),
	}

	return inf
}

func (inf *meshInformer) stopWatchOneKey(key string) {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if watcher, exists := inf.watchers[key]; exists {
		watcher.Close()
		delete(inf.watchers, key)
	}
}

// OnPartOfServiceSpec watches one service's spec by given gjsonPath.
func (inf *meshInformer) OnPartOfServiceSpec(serviceName string, gjsonPath GJSONPath, fn ServiceSpecFunc) error {
	storeKey := layout.ServiceSpecKey(serviceName)
	watcherKey := fmt.Sprintf("service-spec-%s-%s", serviceName, gjsonPath)

	specFunc := func(event Event, value string) bool {
		var serviceSpec *spec.Service
		if event != EventDelete {
			if err := yaml.Unmarshal([]byte(value), serviceSpec); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, serviceSpec)
	}

	return inf.onSpecPart(storeKey, watcherKey, gjsonPath, specFunc)
}

// OnPartOfInstanceSpec watches one service's instance spec by given gjsonPath.
func (inf *meshInformer) OnPartOfInstanceSpec(serviceName, instanceID string, gjsonPath GJSONPath, fn ServicesInstanceSpecFunc) error {
	storeKey := layout.ServiceInstanceSpecKey(serviceName, instanceID)
	watcherKey := fmt.Sprintf("service-instance-spec-%s-%s-%s", serviceName, instanceID, gjsonPath)

	specFunc := func(event Event, value string) bool {
		var instanceSpec *spec.ServiceInstanceSpec
		if event != EventDelete {
			if err := yaml.Unmarshal([]byte(value), instanceSpec); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, instanceSpec)
	}

	return inf.onSpecPart(storeKey, watcherKey, gjsonPath, specFunc)
}

// OnPartOfServiceInstanceStatus watches one service instance status spec by given gjsonPath.
func (inf *meshInformer) OnPartOfServiceInstanceStatus(serviceName, instanceID string, gjsonPath GJSONPath, fn ServiceInstanceStatusFunc) error {
	storeKey := layout.ServiceInstanceStatusKey(serviceName, instanceID)
	watcherKey := fmt.Sprintf("service-instance-status-%s-%s-%s", serviceName, instanceID, gjsonPath)

	specFunc := func(event Event, value string) bool {
		var instanceStatus *spec.ServiceInstanceStatus
		if event != EventDelete {
			if err := yaml.Unmarshal([]byte(value), instanceStatus); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, instanceStatus)
	}

	return inf.onSpecPart(storeKey, watcherKey, gjsonPath, specFunc)
}

// OnPartOfTenantSpec watches one tenant status spec by given gjsonPath.
func (inf *meshInformer) OnPartOfTenantSpec(tenant string, gjsonPath GJSONPath, fn TenantSpecFunc) error {
	storeKey := layout.TenantSpecKey(tenant)
	watcherKey := fmt.Sprintf("tenant-%s", tenant)

	specFunc := func(event Event, value string) bool {
		var tenantSpec *spec.Tenant
		if event != EventDelete {
			if err := yaml.Unmarshal([]byte(value), tenantSpec); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, tenantSpec)
	}

	return inf.onSpecPart(storeKey, watcherKey, gjsonPath, specFunc)
}

// OnSerivceSpecs watches service specs with the prefix.
func (inf *meshInformer) OnSerivceSpecs(servicePrefix string, fn ServiceSpecsFunc) error {
	watcherKey := fmt.Sprintf("prefix-service-%s", servicePrefix)

	specsFunc := func(kvs map[string]string) bool {
		services := make(map[string]*spec.Service)
		for k, v := range kvs {
			service := &spec.Service{}
			if err := yaml.Unmarshal([]byte(v), service); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			services[k] = service
		}

		return fn(services)
	}

	return inf.onSpecs(servicePrefix, watcherKey, specsFunc)
}

// OnServiceInstanceSpecs watches one service all instance specs.
func (inf *meshInformer) OnServiceInstanceSpecs(serviceName string, fn ServiceInstanceSpecsFunc) error {
	instancePrefix := layout.ServiceInstanceSpecPrefix(serviceName)
	watcherKey := fmt.Sprintf("prefix-service-instance-spec-%s", serviceName)

	specsFunc := func(kvs map[string]string) bool {
		instanceSpecs := make(map[string]*spec.ServiceInstanceSpec)
		for k, v := range kvs {
			instanceSpec := &spec.ServiceInstanceSpec{}
			if err := yaml.Unmarshal([]byte(v), instanceSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			instanceSpecs[k] = instanceSpec
		}

		return fn(instanceSpecs)
	}

	return inf.onSpecs(instancePrefix, watcherKey, specsFunc)
}

// OnServiceInstanceStatuses watches service instance statuses with the same prefix.
func (inf *meshInformer) OnServiceInstanceStatuses(serviceName string, fn ServiceInstanceStatusesFunc) error {
	watcherKey := fmt.Sprintf("prefix-service-instance-status-%s", serviceName)
	instacneStatusPrefix := layout.ServiceInstanceStatusPrefix(serviceName)

	specsFunc := func(kvs map[string]string) bool {
		instanceStatuses := make(map[string]*spec.ServiceInstanceStatus)
		for k, v := range kvs {
			instanceStatus := &spec.ServiceInstanceStatus{}
			if err := yaml.Unmarshal([]byte(v), instanceStatus); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			instanceStatuses[k] = instanceStatus
		}

		return fn(instanceStatuses)
	}

	return inf.onSpecs(instacneStatusPrefix, watcherKey, specsFunc)
}

// OnTenantSpecs watches tenant specs with the same prefix.
func (inf *meshInformer) OnTenantSpecs(tenantPrefix string, fn TenantSpecsFunc) error {
	watcherKey := fmt.Sprintf("prefix-tenant-%s", tenantPrefix)

	specsFunc := func(kvs map[string]string) bool {
		tenants := make(map[string]*spec.Tenant)
		for k, v := range kvs {
			tenantSpec := &spec.Tenant{}
			if err := yaml.Unmarshal([]byte(v), tenantSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			tenants[k] = tenantSpec
		}

		return fn(tenants)
	}

	return inf.onSpecs(tenantPrefix, watcherKey, specsFunc)
}

func (inf *meshInformer) comparePart(path GJSONPath, old, new string) bool {
	if path == AllParts {
		return old == new
	}

	oldJSON, err := yamljsontool.YAMLToJSON([]byte(old))
	if err != nil {
		logger.Errorf("BUG: transform yaml %s to json failed: %v", old, err)
		return true
	}

	newJSON, err := yamljsontool.YAMLToJSON([]byte(new))
	if err != nil {
		logger.Errorf("BUG: transform yaml %s to json failed: %v", new, err)
		return true
	}

	return gjson.Get(string(oldJSON), string(path)) == gjson.Get(string(newJSON), string(path))
}

func (inf *meshInformer) onSpecPart(storeKey, watcherKey string, gjsonPath GJSONPath, fn specHandleFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if inf.closed {
		return ErrClosed
	}

	if _, ok := inf.watchers[watcherKey]; ok {
		logger.Errorf("already watched key: %s", watcherKey)
		return ErrAlreadyWatched
	}

	value, err := inf.store.Get(storeKey)
	if err != nil {
		return err
	}
	if value == nil {
		return ErrNotFound
	}

	watcher, err := inf.store.Watcher()
	if err != nil {
		return err
	}

	ch, err := watcher.Watch(storeKey)
	if err != nil {
		return err
	}

	inf.watchers[watcherKey] = watcher

	go inf.watch(ch, watcherKey, *value, gjsonPath, fn)

	return nil
}

func (inf *meshInformer) onSpecs(storePrefix, watcherKey string, fn specsHandleFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if inf.closed {
		return ErrClosed
	}

	if _, exists := inf.watchers[watcherKey]; exists {
		logger.Infof("already watched prefix: %s", watcherKey)
		return ErrAlreadyWatched
	}

	watcher, err := inf.store.Watcher()
	if err != nil {
		return err
	}

	ch, err := watcher.WatchPrefix(storePrefix)
	if err != nil {
		return err
	}

	kvs, err := inf.store.GetPrefix(storePrefix)
	if err != nil {
		return err
	}

	inf.watchers[watcherKey] = watcher

	go inf.watchPrefix(ch, watcherKey, kvs, fn)

	return nil
}

func (inf *meshInformer) Close() {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	for _, watcher := range inf.watchers {
		watcher.Close()
	}

	inf.closed = true
}

func (inf *meshInformer) watch(ch <-chan *string, watcherKey,
	oldValue string, path GJSONPath, fn specHandleFunc) {

	for value := range ch {
		var continueWatch bool

		if value == nil {
			continueWatch = fn(EventDelete, "")
		} else {
			if !inf.comparePart(path, oldValue, *value) {
				oldValue = *value
				continueWatch = fn(EventUpdate, oldValue)
			}
		}

		if !continueWatch {
			inf.stopWatchOneKey(watcherKey)
		}
	}
}

func (inf *meshInformer) watchPrefix(ch <-chan map[string]*string, watcherKey string,
	kvs map[string]string, fn specsHandleFunc) {

	for changedKvs := range ch {
		for k, v := range changedKvs {
			var continueWatch bool

			if v == nil {
				delete(kvs, k)
				continueWatch = fn(kvs)
			} else {
				kvs[k] = *v
				continueWatch = fn(kvs)
			}

			if !continueWatch {
				inf.stopWatchOneKey(watcherKey)
			}
		}
	}
}
