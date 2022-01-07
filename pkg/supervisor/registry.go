/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package supervisor

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocol"
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

		// Status returns its runtime status.
		Status() *Status

		// Close closes itself. It is called by deleting.
		// Supervisor won't call Close for previous generation in Update.
		Close()
	}

	// Status is the universal status for all objects.
	Status struct {
		// ObjectStatus must be a map or struct (empty is allowed),
		// If the ObjectStatus contains field `timestamp`,
		// it will be covered by the top-level Timestamp here.
		ObjectStatus interface{}
		// Timestamp is the global unix timestamp, the object
		// needs not to set it on its own.
		Timestamp int64
	}

	// TrafficObject is the object of Traffic
	TrafficObject interface {
		Object

		// Init initializes the Object.
		Init(superSpec *Spec, muxMapper protocol.MuxMapper)

		// Inherit also initializes the Object.
		// But it needs to handle the lifecycle of the previous generation.
		// So it's own responsibility for the object to inherit and clean the previous generation stuff.
		// The supervisor won't call Close for the previous generation.
		Inherit(superSpec *Spec, previousGeneration Object, muxMapper protocol.MuxMapper)

		// Type return type of object, which can be server type or pipeline type.
		// this method will be called in rawconfigtrafficcontroller.
		Type() ObjectType

		// Protocol return protocol of object. If protocol is context.General, then their spec should be able to
		// check their running protocol.
		Protocol() context.Protocol
	}

	// ProtocolObject is a object that return object's running object
	ProtocolObject interface {
		Protocol() context.Protocol
	}

	// TrafficGate is the object in category of TrafficGate.
	TrafficGate interface {
		TrafficObject
	}

	// Pipeline is the object in category of Pipeline.
	Pipeline interface {
		TrafficObject
	}

	// Controller is the object in category of Controller.
	Controller interface {
		Object

		// Init initializes the Object.
		Init(superSpec *Spec)

		// Inherit also initializes the Object.
		// But it needs to handle the lifecycle of the previous generation.
		// So it's own responsibility for the object to inherit and clean the previous generation stuff.
		// The supervisor won't call Close for the previous generation.
		Inherit(superSpec *Spec, previousGeneration Object)
	}

	// ObjectCategory is the type to classify all objects.
	ObjectCategory string

	// ObjectType is the type of object, server or pipeline
	ObjectType string
)

const (
	// ServerType is ObjectType of server
	ServerType ObjectType = "server"

	// PipelineType is ObjectType of pipeline
	PipelineType ObjectType = "pipeline"
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

	// objectRegistryOrderByDependency is sorted by object dependency.
	// The reason is that object dependencies follow the package imports sequence.
	// It aims to initialize system controllers which depend others in right sequence.
	//
	// FIXME: Do we need an explicit table to specify the dependency.
	// Because it can get the controller without importing its package.
	objectRegistryOrderByDependency = []Object{}
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

func checkTrafficObject(obj TrafficObject) error {
	if obj.Type() != ServerType && obj.Type() != PipelineType {
		return fmt.Errorf("TrafficObject return wrong type %v, not %s or %s", obj.Type(), ServerType, PipelineType)
	}
	if obj.Protocol() == context.General {
		_, ok := obj.DefaultSpec().(ProtocolObject)
		if !ok {
			return fmt.Errorf("TrafficObject return General protocol type should provide Protocol() method for spec to get running protocol")
		}
	} else if _, ok := context.ProtocolMap[obj.Protocol()]; !ok {
		return fmt.Errorf("TrafficObject return unsupported protocol %s", obj.Protocol())
	}
	return nil
}

// Register registers object.
func Register(o Object) {
	if o.Kind() == "" {
		panic(fmt.Errorf("%T: empty kind", o))
	}

	switch o.Category() {
	case CategoryBusinessController, CategorySystemController:
		_, ok := o.(Controller)
		if !ok {
			panic(fmt.Errorf("%s: doesn't implement interface Controller", o.Kind()))
		}
	case CategoryPipeline, CategoryTrafficGate:
		trafficObject, ok := o.(TrafficObject)
		if !ok {
			panic(fmt.Errorf("%s: doesn't implement interface TrafficObject", o.Kind()))
		}
		if err := checkTrafficObject(trafficObject); err != nil {
			panic(fmt.Errorf("%s check failed, %v", o.Kind(), err))
		}
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
	objectRegistryOrderByDependency = append(objectRegistryOrderByDependency, o)
}
