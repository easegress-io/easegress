package informer

import (
	"fmt"
	"sync"

	yamljsontool "github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/cluster"
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

	// AllParts is the scope of the whole structure.
	AllParts GJSONInformScope = ""

	// ScopeServiceObservability  is the scope of service observability.
	ServiceObservability GJSONInformScope = "observability"

	// ScopeServiceResilience is the scope of service resilience.
	ServiceResilience GJSONInformScope = "resilience"

	// GJSONScopeServiceCanary is the scope of service canary.
	ServiceCanary GJSONInformScope = "canary"

	// ServiceLoadBalance is the scope of service loadbalance.
	ServiceLoadBalance GJSONInformScope = "loadBalance"

	// ServiceCircuitBreaker is the scope of service resielience's circuritBreaker part.
	ServiceCircuitBreaker GJSONInformScope = "resilience.circuitBreaker"
)

type (
	// InformEvent is the type of inform event.
	InformEvent string

	// GJSONInformScope is the type of inform scope, in gjson syntax.
	GJSONInformScope string

	specHandleFunc  func(event InformEvent, value string) bool
	specsHandleFunc func(map[string]string) bool

	// ServiceSpecFunc is the handle function when target service's spec changed
	// return value: true , keep on watching
	//			     false, stop watching
	ServiceSpecFunc func(event InformEvent, value *spec.Service) bool

	// ServiceSpecsFunc is the handle function when target services
	// specs with same prefix changed
	// return value: true , keep on watching
	//			     false, stop watching
	ServiceSpecsFunc func(value map[string]*spec.Service) bool

	// ServicesInstanceFunc is the handle function when target service instance's spec changed
	// return value: true , keep on watching
	//			     false, stop watching
	ServicesInstanceFunc func(event InformEvent, value *spec.ServiceInstanceSpec) bool

	// ServiceInstanceSpecsFunc is the handle function when target service instance
	// specs with same prefix changed
	// return value: true , keep on watching
	//			     false, stop watching
	ServiceInstanceSpecsFunc func(value map[string]*spec.ServiceInstanceSpec) bool

	// ServiceInstanceStatusFunc is the handle function when target service instance
	// status spec  changed
	// return value: true , keep on watching
	//			     false, stop watching
	ServiceInstanceStatusFunc func(event InformEvent, value *spec.ServiceInstanceStatus) bool

	// ServiceInstanceStatusesFunc is the handle function when target service instance
	// specs with same prefix changed
	// return value: true , keep on watching
	//			     false, stop watching
	ServiceInstanceStatusesFunc func(value map[string]*spec.ServiceInstanceStatus) bool

	// TenantFunc is the handle function when target tenant's spec changed
	// return value: true , keep on watching
	//			     false, stop watching
	TenantFunc func(event InformEvent, value *spec.Tenant) bool

	// ServiceInstanceStatusSpecsFunc is the handle function when same service instance status records
	// changed.
	// return value: true , keep on watching
	//			     false, stop watching
	TenantSpecsFunc func(value map[string]*spec.Tenant) bool

	// Informer is the interface for informing two type of storage changed for every Mesh spec structure.
	//  1. Based on one record's gjson field compared
	//  2. Based on same prefix's records modification
	Informer interface {
		OnPartOfServiceSpec(serviceName string, gjsonScope GJSONInformScope, fn ServiceSpecFunc) error
		OnSerivceSpecs(servicePrefix string, fn ServiceSpecsFunc) error

		OnPartOfInstanceSpec(serviceName, instanceID string, gjsonScope GJSONInformScope, fn ServicesInstanceFunc) error
		OnServiceInstanceSpecs(serviceName string, fn ServiceInstanceSpecsFunc) error

		OnPartOfServiceInstanceStatus(serviceName, instanceID string, gjsonScope GJSONInformScope, fn ServiceInstanceStatusFunc) error
		OnServiceInstanceStatuses(instanceStatusPrefix string, fn ServiceInstanceStatusesFunc) error

		OnPartOfTenantSpec(tenantName string, gjsonScope GJSONInformScope, fn TenantFunc) error
		OnTenantSpecs(tenantPrefix string, fn TenantSpecsFunc) error

		Close()
	}

	// closer is a closer which handles one watching
	// resource closing
	closer struct {
		done    chan struct{}
		watcher cluster.Watcher
	}

	// meshInformer is the informer for mesh usage
	meshInformer struct {
		store  storage.Storage
		dict   map[string]closer
		closed bool

		done  chan struct{}
		mutex sync.Mutex
	}
)

var (
	// ErrAlreadyWatched is the error when calling informer to watch the same name/prefix again.
	ErrAlreadyWatched = fmt.Errorf("already watched")

	// ErrWatchInClosedInforemer is the error when calling a closed informer to watch.
	ErrWatchInClosedInforemer = fmt.Errorf("informer already been closed")

	// ErrRecordNotFound is the errro when can't find the desired key from storage.
	ErrRecordNotFound = fmt.Errorf("record not found")
)

// NewMeshInformer creates an Informer to watch service event.
func NewMeshInformer(store storage.Storage, done chan struct{}) Informer {
	inf := &meshInformer{
		store:  store,
		dict:   make(map[string]closer),
		mutex:  sync.Mutex{},
		done:   done,
		closed: false,
	}

	go inf.run()
	return inf
}

// Close closes the channel and call storage's watchre's Close()
func (c *closer) Close() {
	close(c.done)
	c.watcher.Close()
}

// Close closes the informer
func (inf *meshInformer) Close() {
	close(inf.done)
}

// StopWatchOneKey will call one key's closer to close and then delete it from
// informer's dictionary
func (inf *meshInformer) StopWatchOneKey(dictName string) {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if v, ok := inf.dict[dictName]; ok {
		v.Close()
		delete(inf.dict, dictName)
	}
	return
}

// OnPartOfServiceSpec watchs one service's spec by given gjsonScope
func (inf *meshInformer) OnPartOfServiceSpec(serviceName string, gjsonScope GJSONInformScope, fn ServiceSpecFunc) error {
	storeKey := layout.ServiceSpecKey(serviceName)
	dictKey := fmt.Sprintf("service-%s-%s", serviceName, gjsonScope)

	specFunc := func(event InformEvent, value string) bool {
		var service spec.Service
		if err := yaml.Unmarshal([]byte(value), service); err != nil {
			if err != nil {
				logger.Errorf("BUG, informer  yaml unmsarshal service:[%s] failed, err:%v", value, err)
				// keep on watching
				return true
			}
		}
		return fn(event, &service)
	}

	return inf.onSpecPart(storeKey, dictKey, gjsonScope, specFunc)
}

// OnPartOfInstanceSpec watchs one service's instance spec by given gjsonScope
func (inf *meshInformer) OnPartOfInstanceSpec(serviceName, instanceID string, gjsonScope GJSONInformScope, fn ServicesInstanceFunc) error {
	dictKey := fmt.Sprintf("service-instance-%s-%s-%s", serviceName, instanceID, gjsonScope)
	storeKey := layout.ServiceInstanceSpecKey(serviceName, instanceID)

	specFunc := func(event InformEvent, value string) bool {
		var ins spec.ServiceInstanceSpec
		if err := yaml.Unmarshal([]byte(value), &ins); err != nil {
			if err != nil {
				logger.Errorf("BUG, informer  yaml unmsarshal service:[%s], instacne :[%s] failed, err:%v", serviceName, instanceID, value, err)
				return true
			}
		}
		return fn(event, &ins)
	}

	return inf.onSpecPart(storeKey, dictKey, gjsonScope, specFunc)
}

// OnPartOfServiceInstanceStatus watches one service instance status spec by given gjsonScope
func (inf *meshInformer) OnPartOfServiceInstanceStatus(serviceName, instanceID string, gjsonScope GJSONInformScope, fn ServiceInstanceStatusFunc) error {
	storeKey := layout.ServiceInstanceStatusKey(serviceName, instanceID)
	dictKey := fmt.Sprintf("service-instance-status-%s", serviceName)

	specFunc := func(event InformEvent, value string) bool {
		var ins spec.ServiceInstanceStatus
		if err := yaml.Unmarshal([]byte(value), &ins); err != nil {
			if err != nil {
				logger.Errorf("BUG, informer  yaml unmsarshal service:[%s], instacne status:[%s] failed, err:%v", serviceName, instanceID, value, err)
				return true
			}
		}
		return fn(event, &ins)
	}

	return inf.onSpecPart(storeKey, dictKey, gjsonScope, specFunc)
}

// OnPartOfTenantSpec watches one tenant status spec by gieven gjsonScope
func (inf *meshInformer) OnPartOfTenantSpec(tenant string, gjsonScope GJSONInformScope, fn TenantFunc) error {
	storeKey := layout.TenantSpecKey(tenant)
	dictKey := fmt.Sprintf("tenant-%s", tenant)

	specFunc := func(event InformEvent, value string) bool {
		var t spec.Tenant
		if err := yaml.Unmarshal([]byte(value), &t); err != nil {
			if err != nil {
				logger.Errorf("BUG, informer  yaml unmsarshal tenant:[%s], value:[%s] failed, err:%v", tenant, value, err)
				return true
			}
		}
		return fn(event, &t)
	}

	return inf.onSpecPart(storeKey, dictKey, gjsonScope, specFunc)
}

// OnSerivceSpecs watches serveral services' specs by given prefix
func (inf *meshInformer) OnSerivceSpecs(servicePrefix string, fn ServiceSpecsFunc) error {
	dictKey := fmt.Sprintf("service-prefix-%s", servicePrefix)

	specsFunc := func(services map[string]string) bool {
		var svcMap map[string]*spec.Service = make(map[string]*spec.Service)
		for k, v := range services {
			var service spec.Service
			if err := yaml.Unmarshal([]byte(v), &service); err != nil {
				logger.Errorf("BUG, informer yaml unmsarshal service:[%s], val:[%s], failed, err:%v", k, v, err)
				continue
			}
			svcMap[k] = &service
		}

		return fn(svcMap)
	}

	return inf.OnSpecs(servicePrefix, dictKey, specsFunc)
}

// OnServiceInstanceSpecs watchs one service all instance recorders
func (inf *meshInformer) OnServiceInstanceSpecs(serviceName string, fn ServiceInstanceSpecsFunc) error {
	instancePrefix := layout.ServiceInstanceSpecPrefix(serviceName)
	dictKey := fmt.Sprintf("service-instance-%s", serviceName)

	specsFunc := func(serviceInstances map[string]string) bool {
		var insMap map[string]*spec.ServiceInstanceSpec = make(map[string]*spec.ServiceInstanceSpec)
		for k, v := range serviceInstances {
			var serviceIns spec.ServiceInstanceSpec
			if err := yaml.Unmarshal([]byte(v), &serviceIns); err != nil {
				logger.Errorf("BUG, informer yaml unmsarshal service:[%s], instance:[%s],val:[%s], failed, err:%v", serviceName, k, v, err)
				continue
			}
			insMap[k] = &serviceIns
		}

		return fn(insMap)
	}

	return inf.OnSpecs(instancePrefix, dictKey, specsFunc)
}

// OnServiceInstanceStatuses watchs service instance status recorders with same prefix
func (inf *meshInformer) OnServiceInstanceStatuses(instanceStatusPrefix string, fn ServiceInstanceStatusesFunc) error {
	dictKey := fmt.Sprintf("service-instance-status-prefix-%s", instanceStatusPrefix)

	specsFunc := func(serviceInstanceStatuses map[string]string) bool {
		var insMap map[string]*spec.ServiceInstanceStatus = make(map[string]*spec.ServiceInstanceStatus)
		for k, v := range serviceInstanceStatuses {
			var serviceInsStatus spec.ServiceInstanceStatus
			if err := yaml.Unmarshal([]byte(v), &serviceInsStatus); err != nil {
				logger.Errorf("BUG, informer yaml unmsarshal service instance prefix:[%s], instancestatus:[%s],val:[%s], failed, err:%v", instanceStatusPrefix, k, v, err)
				continue
			}
			insMap[k] = &serviceInsStatus
		}

		return fn(insMap)
	}

	return inf.OnSpecs(instanceStatusPrefix, dictKey, specsFunc)
}

// OnTenantSpecs watchs tenants recorders with the same prefix
func (inf *meshInformer) OnTenantSpecs(tenantPrefix string, fn TenantSpecsFunc) error {
	dictKey := fmt.Sprintf("tenant-prefix-%s", tenantPrefix)

	specsFunc := func(tenants map[string]string) bool {
		var tenantMap map[string]*spec.Tenant = make(map[string]*spec.Tenant)
		for k, v := range tenants {
			var tenant spec.Tenant
			if err := yaml.Unmarshal([]byte(v), &tenant); err != nil {
				logger.Errorf("BUG, informer yaml unmsarshal tenant prrefix:[%s], tenant:[%s],val:[%s], failed, err:%v", tenantPrefix, k, v, err)
				continue
			}
			tenantMap[k] = &tenant
		}

		return fn(tenantMap)
	}

	return inf.OnSpecs(tenantPrefix, dictKey, specsFunc)
}

func (inf *meshInformer) compare(scope GJSONInformScope, origin, new string) bool {
	originSvc, err := yamljsontool.YAMLToJSON([]byte(origin))
	if err != nil {
		logger.Errorf("BUG, mesh informer using yamljsontool YAMLtoJSON faile, val:%s, err:v", origin, err)
		return true
	}

	newSvc, err := yamljsontool.YAMLToJSON([]byte(new))
	if err != nil {
		logger.Errorf("BUG, mesh informer using yamljsontool YAMLtoJSON faile, val:%s, err:v", new, err)
		return true
	}

	// compare the whole json string
	if scope == AllParts {
		return string(originSvc) == string(newSvc)
	}

	// using gjson to extract desired field
	if gjson.Get(string(originSvc), string(scope)) == gjson.Get(string(newSvc), string(scope)) {
		return true
	}

	return false
}

// onSpecPart will call back fn when given scope of field change in desired structure's JSON
// (describe by gsjsonScope parameter).
// Scope select support gjson syntax.
func (inf *meshInformer) onSpecPart(storeKey, dictKey string, gjsonScope GJSONInformScope, fn specHandleFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()
	if inf.closed == true {
		return ErrWatchInClosedInforemer
	}

	if _, ok := inf.dict[dictKey]; ok {
		logger.Infof("already watching key:%s", dictKey)
		return ErrAlreadyWatched
	}

	var (
		s   *string
		err error
	)
	if s, err = inf.store.Get(storeKey); err != nil {
		return err
	}
	if len(*s) == 0 {
		return ErrRecordNotFound
	}

	watcher, err := inf.store.Watcher()
	if err != nil {
		return err
	}

	ch, err := watcher.Watch(storeKey)
	if err != nil {
		return err
	}

	closeChan := make(chan struct{})
	inf.dict[dictKey] = closer{
		done:    closeChan,
		watcher: watcher,
	}
	go inf.watch(ch, *s, dictKey, gjsonScope, fn, closeChan)

	return nil
}

// OnSpecs will call back fn when given name service's instance list changed
func (inf *meshInformer) OnSpecs(storePrefix, dictKey string, fn specsHandleFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if inf.closed == true {
		return ErrWatchInClosedInforemer
	}
	if _, ok := inf.dict[dictKey]; ok {
		logger.Infof("already wathed prefix:%s", dictKey)
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

	var originMap map[string]string = make(map[string]string)
	if originMap, err = inf.store.GetPrefix(storePrefix); err != nil {
		return err
	}

	closeChan := make(chan struct{})
	inf.dict[dictKey] = closer{
		done:    closeChan,
		watcher: watcher,
	}
	go inf.watchPrefix(ch, dictKey, originMap, fn, closeChan)

	return nil
}

func (inf *meshInformer) run() {
	for {
		select {
		case <-inf.done:
			inf.mutex.Lock()
			defer inf.mutex.Unlock()
			// close all watching goroutine
			for _, v := range inf.dict {
				v.Close()
			}
			inf.closed = true
			return
		}
	}
}

func (inf *meshInformer) watch(ch <-chan *string, dictKey, specYAML string, scope GJSONInformScope, fn specHandleFunc, closeChan chan struct{}) {
	for {
		select {
		case <-closeChan:
			logger.Infof("watching dictKey :%s closed", dictKey)
			return
		case newSpecYAML := <-ch:
			if newSpecYAML == nil {
				if fn(EventDelete, "") == false {
					// handler want to cancle watching this key
					inf.StopWatchOneKey(dictKey)
				}
			} else {
				if !inf.compare(scope, specYAML, *newSpecYAML) {
					specYAML = *newSpecYAML
					if fn(EventUpdate, specYAML) == false {
						// handler want to cancle watching this key
						inf.StopWatchOneKey(dictKey)
					}
				}
			}
		}
	}

}

func (inf *meshInformer) watchPrefix(ch <-chan map[string]*string, dictKey string, originMap map[string]string, fn specsHandleFunc, closeChan chan struct{}) {
	for {
		select {
		case <-closeChan:
			logger.Infof("watching prefix %s closed", dictKey)
			return
		case changeMap := <-ch:
			// only one key in changeMap
			for k, v := range changeMap {
				// delete
				if v == nil {
					if _, ok := originMap[k]; ok {
						delete(originMap, k)
						if fn(originMap) == false {
							inf.StopWatchOneKey(dictKey)
						}
					}
				} else {
					// add or update
					originMap[k] = *v
					if fn(originMap) == false {
						inf.StopWatchOneKey(dictKey)
					}
				}
			}
		}
	}
}
