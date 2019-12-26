package httppipeline

import (
	"fmt"
	"reflect"

	"github.com/megaease/easegateway/pkg/context"
)

type (
	// Plugin is the common interface for plugins handling HTTP traffic.
	// All Plugins need to implement Handle and Close.
	//
	// Every Plugin registers itself in its package function init().
	// It must give the information below:
	//
	// 1. Kind: A unique name represented its kind.
	// 2. DefaultSpecFunc: A function returns its default Spec.
	//   2.1 Spec must be a struct with two string fields: Name, Kind.
	// 3. NewFunc: A function returns its running instance.
	//   3.1 First input argument must be the type of its Spec.
	//   3.2 Second input argument must be the type of itself,
	//       which is its previous generation after Spec updated.
	//   3.3 The one and only one output argument is the type of itself.
	// 4. Results: All possible results the Handle would return.
	//   4.1 No need to register empty string which won't
	//     break the HTTPPipeline by default.
	//
	// And the registry will check more for the Plugin itself.
	// 1. It must implement function Status
	//   1.1 It has one and only one output argument in any types.
	Plugin interface {
		Handle(context.HTTPContext) (result string)
		Close()
	}

	// PluginMeta describes metadata of Plugin.
	PluginMeta struct {
		Name string `yaml:"name,omitempty" jsonschema:"omitempty,format=urlname"`
		Kind string `yaml:"kind,omitempty" jsonschema:"omitempty"`
	}

	// PluginRecord is the record for booking plugin.
	PluginRecord struct {
		Kind string
		// func DefaultSpec() *PluginSpec
		DefaultSpecFunc interface{}
		// func New(spec *PluginSpec, prev *Plugin) *Plugin
		NewFunc interface{}
		Results []string

		Description string

		PluginType reflect.Type
		SpecType   reflect.Type
	}
)

var (
	pluginBook = map[string]*PluginRecord{}
)

func (pr *PluginRecord) copy() *PluginRecord {
	pr1 := *pr
	results := make([]string, len(pr.Results))
	reflect.Copy(reflect.ValueOf(results), reflect.ValueOf(pr.Results))
	pr1.Results = results
	return &pr1
}

// Register registers plugins scheduled by HTTPPipeline.
func Register(pr *PluginRecord) {
	if pr.Kind == "" {
		panic("empty kind")
	}

	assert := func(x, y interface{}, err error) {
		if !reflect.DeepEqual(x, y) {
			panic(fmt.Errorf("%s: %v", pr.Kind, err))
		}
	}
	assertFunc := func(name string, t reflect.Type, numIn, numOut int) {
		assert(t.Kind(), reflect.Func, fmt.Errorf("%s: not func", name))
		assert(t.NumIn(), numIn, fmt.Errorf("%s: input arguments: want %d in, got %d", name, numIn, t.NumIn()))
		assert(t.NumOut(), numOut, fmt.Errorf("%s: input arguments: want %d in, got %d", name, numOut, t.NumOut()))
	}

	prExisted, exists := pluginBook[pr.Kind]
	assert(exists, false, fmt.Errorf("conflict kind: %s: %#v", pr.Kind, prExisted))

	// SpecFunc
	specFuncType := reflect.TypeOf(pr.DefaultSpecFunc)
	assertFunc("DefaultSpecFunc", specFuncType, 0, 1)

	// Spec
	pr.SpecType = specFuncType.Out(0)
	assert(pr.SpecType.Kind(), reflect.Ptr, fmt.Errorf("non pointer spec"))
	assert(pr.SpecType.Elem().Kind(), reflect.Struct,
		fmt.Errorf("non struct spec elem: %s", pr.SpecType.Elem().Kind()))
	nameField, exists := pr.SpecType.Elem().FieldByName("Name")
	assert(exists, true, fmt.Errorf("no Name field in spec"))
	assert(nameField.Type.Kind(), reflect.String, fmt.Errorf("Name field which is not string"))
	kindField, exists := pr.SpecType.Elem().FieldByName("Kind")
	assert(exists, true, fmt.Errorf("no Kind field in spec"))
	assert(kindField.Type.Kind(), reflect.String, fmt.Errorf("Kind field which is not string"))

	// NewFunc
	newFuncType := reflect.TypeOf(pr.NewFunc)
	assertFunc("NewFunc", newFuncType, 2, 1)
	assert(newFuncType.In(0), pr.SpecType,
		fmt.Errorf("conflict NewFunc and DefaultSpecFunc: "+
			"1st input argument of NewFunc is different type from "+
			"output argument of DefaultSpecFunc"))
	assert(newFuncType.In(1), newFuncType.Out(0),
		fmt.Errorf("invalid NewFunc "+
			"2nd input argument is different type from output argument of NewFunc"))

	// Plugin
	pr.PluginType = newFuncType.Out(0)
	pluginType := reflect.TypeOf((*Plugin)(nil)).Elem()
	assert(pr.PluginType.Implements(pluginType), true,
		fmt.Errorf("invalid plugin: not implement httppipeline.Plugin"))

	// StatusFunc
	statusMethod, exists := pr.PluginType.MethodByName("Status")
	assert(exists, true, fmt.Errorf("no func Status"))
	// NOTE: Method always has more than one argument, the first one is the receiver.
	assertFunc("Status", statusMethod.Type, 1, 1)

	// Results
	results := make(map[string]struct{})
	for _, result := range pr.Results {
		assert(result == "", false, fmt.Errorf("empty result"))

		_, exists := results[result]
		assert(exists, false, fmt.Errorf("repeated result: %s", result))
		results[result] = struct{}{}
	}

	pluginBook[pr.Kind] = pr
}

// GetPluginBook copies the plugin book.
func GetPluginBook() map[string]*PluginRecord {
	result := map[string]*PluginRecord{}

	for kind, pr := range pluginBook {
		result[kind] = pr.copy()
	}

	return result
}
