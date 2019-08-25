package httppipeline

import (
	"fmt"
	"reflect"
)

type (
	// PluginRecord is the record for booking plugin.
	PluginRecord struct {
		Kind string
		// func DefaultSpec() *PluginSpec
		DefaultSpecFunc interface{}
		// func New(spec *PluginSpec, prev *Plugin) *Plugin
		NewFunc interface{}
		Results []string

		pluginType reflect.Type
		specType   reflect.Type
	}
)

var (
	pluginBook = map[string]*PluginRecord{}
)

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
	pr.specType = specFuncType.Out(0)
	assert(pr.specType.Kind(), reflect.Ptr, fmt.Errorf("non pointer spec"))
	assert(pr.specType.Elem().Kind(), reflect.Struct,
		fmt.Errorf("non struct spec elem: %s", pr.specType.Elem().Kind()))
	nameField, exists := pr.specType.Elem().FieldByName("Name")
	assert(exists, true, fmt.Errorf("no Name field in spec"))
	assert(nameField.Type.Kind(), reflect.String, fmt.Errorf("Name field which is not string"))
	kindField, exists := pr.specType.Elem().FieldByName("Kind")
	assert(exists, true, fmt.Errorf("no Kind field in spec"))
	assert(kindField.Type.Kind(), reflect.String, fmt.Errorf("Kind field which is not string"))

	// NewFunc
	newFuncType := reflect.TypeOf(pr.NewFunc)
	assertFunc("NewFunc", newFuncType, 2, 1)
	assert(newFuncType.In(0), pr.specType,
		fmt.Errorf("conflict NewFunc and DefaultSpecFunc: "+
			"1st input argument of NewFunc is different type from "+
			"output argument of DefaultSpecFunc"))
	assert(newFuncType.In(1), newFuncType.Out(0),
		fmt.Errorf("invalid NewFunc "+
			"2nd input argument is different type from output argument of NewFunc"))

	// Plugin
	pr.pluginType = newFuncType.Out(0)
	pluginType := reflect.TypeOf((*Plugin)(nil)).Elem()
	assert(pr.pluginType.Implements(pluginType), true,
		fmt.Errorf("invalid plugin: not implement httppipeline.Plugin"))

	// StatusFunc
	statusMethod, exists := pr.pluginType.MethodByName("Status")
	assert(exists, true, fmt.Errorf("no func Status"))
	// NOTE: Method always has more than one argument, the first one is the receiver.
	assertFunc("Status", statusMethod.Type, 1, 1)

	// Results
	results := make(map[string]struct{})
	for _, result := range pr.Results {
		_, exists := results[result]
		assert(exists, false, fmt.Errorf("repeated result: %s", result))
		results[result] = struct{}{}
	}

	pluginBook[pr.Kind] = pr
}
