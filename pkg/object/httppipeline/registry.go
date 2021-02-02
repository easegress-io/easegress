package httppipeline

import (
	"fmt"
	"reflect"

	"github.com/megaease/easegateway/pkg/context"
)

type (
	// Filter is the common interface for filters handling HTTP traffic.
	// All Filters need to implement Handle and Close.
	//
	// Every Filter registers itself in its package function init().
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
	// And the registry will check more for the Filter itself.
	// 1. It must implement function Status
	//   1.1 It has one and only one output argument in any types.
	Filter interface {
		Handle(context.HTTPContext) (result string)
		Close()
	}

	// FilterMeta describes metadata of Filter.
	FilterMeta struct {
		Name string `yaml:"name,omitempty" jsonschema:"omitempty,format=urlname"`
		Kind string `yaml:"kind,omitempty" jsonschema:"omitempty"`
	}

	// FilterRecord is the record for booking filter.
	FilterRecord struct {
		Kind string
		// func DefaultSpec() *FilterSpec
		DefaultSpecFunc interface{}
		// func New(spec *FilterSpec, prev *Filter) *Filter
		NewFunc interface{}
		Results []string

		Description string

		FilterType reflect.Type
		SpecType   reflect.Type
	}
)

var (
	filterBook = map[string]*FilterRecord{}
)

func (fr *FilterRecord) copy() *FilterRecord {
	fr1 := *fr
	results := make([]string, len(fr.Results))
	reflect.Copy(reflect.ValueOf(results), reflect.ValueOf(fr.Results))
	fr1.Results = results
	return &fr1
}

// Register registers filters scheduled by HTTPPipeline.
func Register(fr *FilterRecord) {
	if fr.Kind == "" {
		panic("empty kind")
	}

	assert := func(x, y interface{}, err error) {
		if !reflect.DeepEqual(x, y) {
			panic(fmt.Errorf("%s: %v", fr.Kind, err))
		}
	}
	assertFunc := func(name string, t reflect.Type, numIn, numOut int) {
		assert(t.Kind(), reflect.Func, fmt.Errorf("%s: not func", name))
		assert(t.NumIn(), numIn, fmt.Errorf("%s: input arguments: want %d, got %d", name, numIn, t.NumIn()))
		assert(t.NumOut(), numOut, fmt.Errorf("%s: output arguments: want %d, got %d", name, numOut, t.NumOut()))
	}

	prExisted, exists := filterBook[fr.Kind]
	assert(exists, false, fmt.Errorf("conflict kind: %s: %#v", fr.Kind, prExisted))

	// SpecFunc
	specFuncType := reflect.TypeOf(fr.DefaultSpecFunc)
	assertFunc("DefaultSpecFunc", specFuncType, 0, 1)

	// Spec
	fr.SpecType = specFuncType.Out(0)
	assert(fr.SpecType.Kind(), reflect.Ptr, fmt.Errorf("non pointer spec"))
	assert(fr.SpecType.Elem().Kind(), reflect.Struct,
		fmt.Errorf("non struct spec elem: %s", fr.SpecType.Elem().Kind()))
	nameField, exists := fr.SpecType.Elem().FieldByName("Name")
	assert(exists, true, fmt.Errorf("no Name field in spec"))
	assert(nameField.Type.Kind(), reflect.String, fmt.Errorf("Name field which is not string"))
	kindField, exists := fr.SpecType.Elem().FieldByName("Kind")
	assert(exists, true, fmt.Errorf("no Kind field in spec"))
	assert(kindField.Type.Kind(), reflect.String, fmt.Errorf("Kind field which is not string"))

	// NewFunc
	newFuncType := reflect.TypeOf(fr.NewFunc)
	assertFunc("NewFunc", newFuncType, 2, 1)
	assert(newFuncType.In(0), fr.SpecType,
		fmt.Errorf("conflict NewFunc and DefaultSpecFunc: "+
			"1st input argument of NewFunc is different type from "+
			"output argument of DefaultSpecFunc"))
	assert(newFuncType.In(1), newFuncType.Out(0),
		fmt.Errorf("invalid NewFunc "+
			"2nd input argument is different type from output argument of NewFunc"))

	// Filter
	fr.FilterType = newFuncType.Out(0)
	filterType := reflect.TypeOf((*Filter)(nil)).Elem()
	assert(fr.FilterType.Implements(filterType), true,
		fmt.Errorf("invalid filter: not implement httppipeline.Filter"))

	// StatusFunc
	statusMethod, exists := fr.FilterType.MethodByName("Status")
	assert(exists, true, fmt.Errorf("no func Status"))
	// NOTE: Method always has more than one argument, the first one is the receiver.
	assertFunc("Status", statusMethod.Type, 1, 1)

	// Results
	results := make(map[string]struct{})
	for _, result := range fr.Results {
		assert(result == "", false, fmt.Errorf("empty result"))

		_, exists := results[result]
		assert(exists, false, fmt.Errorf("repeated result: %s", result))
		results[result] = struct{}{}
	}

	filterBook[fr.Kind] = fr
}

// GetFilterBook copies the filter book.
func GetFilterBook() map[string]*FilterRecord {
	result := map[string]*FilterRecord{}

	for kind, fr := range filterBook {
		result[kind] = fr.copy()
	}

	return result
}
