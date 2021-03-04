package informer

import "github.com/megaease/easegateway/pkg/object/meshcontroller/storage"

const (
	// EventAdd is the add infrom event.
	EventAdd InformEvent = "Add"
	// EventUpdate is the update infrom event.
	EventUpdate = "Update"
	// EventDelete is the delete infrom event.
	EventDelete = "Delete"

	// ScopeService is the scope of service.
	ScopeService InformScope = "ScopeSericeSpec"

	// ScopeServiceResilience is the scope of service resilience.
	ScopeServiceResilience = "ScopeServiceResilience"

	// ScopeServiceCircuitBreaker is the scope of service circuit breaker.
	ScopeServiceCircuitBreaker = "ScopeServiceCircuitBreaker"

	// ScopeServiceCanary is the scope of service canary.
	ScopeServiceCanary = "ScopeServiceCanary"

	// ScopeServiceIntanceHearbeat is the scope of service instance heartbeat.
	ScopeServiceIntanceHearbeat = "ScopeServiceInstanceHeartbeat"

	// ScopeTenant is the scope of tenant.
	ScopeTenant = "ScopeTenant"
)

type (
	// InformEvent is the type of inform event.
	InformEvent string

	// InformScope is the type of inform scope.
	InformScope string

	// InformFunc is the type of inform function.
	// value is empty when event is DeleteEvent.
	InformFunc func(event InformEvent, value string)

	// Informer is the dedicated informer for Mesh Controller.
	Informer interface {
		// OnScope registers callback function which will be called
		// within specified scope.
		// name could be different meanings in different scopes.
		// e.g. service name in ServiceScope
		//	tenant name in TenantScope
		OnScope(name string, scope InformScope, fn InformFunc)
	}

	informer struct {
		store storage.Storage
	}
)

// NewInformer creates an Informer.
func NewInformer(store storage.Storage) Informer {
	return &informer{
		store: store,
	}
}

func (inf *informer) OnScope(name string, scope InformScope, fn InformFunc) {
	// TODO
}
