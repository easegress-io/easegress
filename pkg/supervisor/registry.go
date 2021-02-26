package supervisor

import (
	"fmt"
	"reflect"
	"sort"
)

type (
	// Object is the common interface for all objects whose lifecycle supervisor handles.
	Object interface {
		// Category returns the object category of itself.
		Category() ObjectCategory

		// Kind returns the unique kind name to represent itself.
		Kind() string

		// DefaultSpec returns the default spec.
		// It must return a pointer to point a struct.
		DefaultSpec() interface{}

		// Init initializes the Obejct.
		Init(superSpec *Spec, super *Supervisor)

		// Inherit also initializes the Object.
		// But it needs to handle the lifecycle of the previous generation.
		// So it's own responsibility for the object to inherit and clean the previous generation stuff.
		// The supervisor won't call Close for the previous generation.
		Inherit(superSpec *Spec, previousGeneration Object, super *Supervisor)

		// Status returns its runtime status.
		Status() *Status

		// Close closes itself. It is called by deleting.
		// Supervisor won't call Close for previous generation in Update.
		Close()
	}

	// Status is the universal status for all objects.
	Status struct {
		// If the ObjectStatus contains field `timestamp`,
		// it will be covered by the top-level Timestamp here.
		ObjectStatus interface{}
		// Timestamp is the global unix timestamp, the object
		// needs not to set it on its own.
		Timestamp int64
	}

	// TrafficGate is the object in category of TrafficGate.
	TrafficGate interface {
		Object
	}

	// Pipeline is the object in category of Pipeline.
	Pipeline interface {
		Object
	}

	// Controller is the object in category of Controller.
	Controller interface {
		Object
	}

	// ObjectCategory is the type to classify all objects.
	ObjectCategory string
)

const (
	// CategoryAll is just for filter of search.
	CategoryAll ObjectCategory = ""
	// CategorySystemController is the category of system controller.
	CategorySystemController = "SystemController"
	// CategoryBusinessController is the category of business controller.
	CategoryBusinessController = "BusinessController"
	// CategoryPipeline is the category of pipeline.
	CategoryPipeline = "Pipeline"
	// CategoryTrafficGate is the category of traffic gate.
	CategoryTrafficGate = "TrafficGate"
)

var (
	// objectCategories is sorted in priority.
	// Which means CategorySystemController is higher than CategoryTrafficGate in priority.
	// So the starting sequence is the same with the array,
	// and the closing sequence is on the contrary
	objectOrderedCategories = []ObjectCategory{
		CategorySystemController,
		CategoryBusinessController,
		CategoryPipeline,
		CategoryTrafficGate,
	}

	// key: kind
	objectRegistry = map[string]Object{}
)

// ObjectKinds returns all object kinds.
func ObjectKinds() []string {
	kinds := make([]string, 0)
	for _, o := range objectRegistry {
		kinds = append(kinds, o.Kind())
	}

	sort.Strings(kinds)

	return kinds
}

// Register registers object.
func Register(o Object) {
	if o.Kind() == "" {
		panic(fmt.Errorf("%T: empty kind", o))
	}

	existedObject, existed := objectRegistry[o.Kind()]
	if existed {
		panic(fmt.Errorf("%T and %T got same kind: %s", o, existedObject, o.Kind()))
	}

	// Checking category.
	foundCategory := false
	for _, category := range objectOrderedCategories {
		if category == o.Category() {
			foundCategory = true
		}
	}
	if !foundCategory {
		panic(fmt.Errorf("%s: unsupported category: %s", o.Kind(), o.Category()))
	}

	// Checking object type.
	objectType := reflect.TypeOf(o)
	if objectType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("%s: want a pointer, got %s", o.Kind(), objectType.Kind()))
	}
	if objectType.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("%s elem: want a struct, got %s", o.Kind(), objectType.Kind()))
	}

	// Checking spec type.
	specType := reflect.TypeOf(o.DefaultSpec())
	if specType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("%s spec: want a pointer, got %s", o.Kind(), specType.Kind()))
	}
	if specType.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("%s spec elem: want a struct, got %s", o.Kind(), specType.Elem().Kind()))
	}

	objectRegistry[o.Kind()] = o
}
