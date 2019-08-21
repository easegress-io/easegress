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
		// func New(spec *PluginSpecev *Plugin) *Plugin
		NewFunc interface{}
		Results []string

		pluginType reflect.Type
		specType   reflect.Type

		needPrev   bool
		needClose  bool
		needStatus bool
	}
)

var (
	pluginBook = map[string]*PluginRecord{}
)

// Register registers plugins scheduled by HTTPPipeline.
func Register(pr *PluginRecord) {
	defer func() {
		if err := recover(); err != nil {
			panic(fmt.Errorf("BUG: %s: %v", pr.Kind, err))
		}
	}()

	if pr.Kind == "" {
		panic("empty kind")
	}
	if prExisted, exists := pluginBook[pr.Kind]; exists {
		panic(fmt.Errorf("%#v already existed, conflict kind",
			prExisted))
	}

	// NOTE: Validate spec.
	specFuncType := reflect.TypeOf(pr.DefaultSpecFunc)
	if specFuncType.Kind() != reflect.Func {
		panic("invalid DefaultSpecFunc")
	}
	if specFuncType.NumIn() != 0 {
		panic("invalid DefaultSpecFunc: not 0 input argument")
	}
	if specFuncType.NumOut() != 1 {
		panic("invalid DefaultSpecFunc: not 1 input argument")
	}
	pr.specType = specFuncType.Out(0)
	if pr.specType.Kind() != reflect.Ptr {
		panic("non pointer spec")
	}
	if pr.specType.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("non struct pointer spec: %s", pr.specType.Kind()))
	}
	nameField, exists := pr.specType.Elem().FieldByName("Name")
	if !exists {
		panic("no Name field in spec")
	}
	if nameField.Type.Kind() != reflect.String {
		panic("Name field which is not string")
	}
	kindField, exists := pr.specType.Elem().FieldByName("Kind")
	if !exists {
		panic("no Kind field in spec")
	}
	if kindField.Type.Kind() != reflect.String {
		panic("Kind field which is not string")
	}

	// NOTE: Validate plugin.
	newFuncType := reflect.TypeOf(pr.NewFunc)
	if newFuncType.Kind() != reflect.Func {
		panic("invalid NewFunc")
	}
	if newFuncType.NumOut() != 1 {
		panic("invalid NewFunc: not 1 output arguments")

	}
	switch newFuncType.NumIn() {
	case 1, 2:
		if newFuncType.In(0) != specFuncType.Out(0) {
			panic("conflict NewFunc and DefaultSpecFunc: " +
				"first input argument of NewFunc is different type from " +
				"output argument of DefaultSpecFunc")
		}
		if newFuncType.NumIn() == 2 {
			if newFuncType.In(1) != newFuncType.Out(0) {
				panic("invalid newFunc " +
					"second input argument is different type from output argument of NewFunc")
			}
			pr.needPrev = true
		}
	default:
		panic("invalid NewFunc: neither 1 nor 2 input arguments")
	}

	// NOTE: Validate methods of plugin.
	outType := newFuncType.Out(0)
	pluginType := reflect.TypeOf((*Plugin)(nil)).Elem()
	if !outType.Implements(pluginType) {
		panic("invalid plugin: not implement httppipeline.Plugin")
	}
	closerType := reflect.TypeOf((*Closer)(nil)).Elem()
	if outType.Implements(closerType) {
		pr.needClose = true
	}
	pr.pluginType = outType

	if method, needStatus := outType.MethodByName("Status"); needStatus {
		if method.Type.NumOut() != 1 {
			panic("invalid func Status with not 1 output arguments")
		}
		pr.needStatus = true
	}

	// NOTE: Validate results.
	results := make(map[string]struct{})
	for _, result := range pr.Results {
		if _, exists := results[result]; exists {
			panic(fmt.Errorf("repeated result: %s", result))

		}
		results[result] = struct{}{}
	}

	pluginBook[pr.Kind] = pr
}
