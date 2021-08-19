package serviceregistry

import (
	"reflect"
	"testing"
)

func TestNewRegistryEventFromDiff(t *testing.T) {
	olds := []*ServiceInstanceSpec{
		{
			RegistryName: "registry-test1",
			ServiceName:  "service-test1",
			InstanceID:   "instance-test1",
		},
	}

	news := []*ServiceInstanceSpec{
		{
			RegistryName: "registry-test1",
			ServiceName:  "service-test1",
			InstanceID:   "instance-test2",
		},
	}

	oldSpecs := map[string]*ServiceInstanceSpec{}
	for _, oldSpec := range olds {
		oldSpecs[oldSpec.Key()] = oldSpec
	}

	newSpecs := map[string]*ServiceInstanceSpec{}
	for _, newSpec := range news {
		newSpecs[newSpec.Key()] = newSpec
	}

	wantEvent := &RegistryEvent{
		SourceRegistryName: "registry-test1",
		Delete: map[string]*ServiceInstanceSpec{
			"registry-test1/service-test1/instance-test1": {
				RegistryName: "registry-test1",
				ServiceName:  "service-test1",
				InstanceID:   "instance-test1",
			},
		},

		Apply: map[string]*ServiceInstanceSpec{
			"registry-test1/service-test1/instance-test2": {
				RegistryName: "registry-test1",
				ServiceName:  "service-test1",
				InstanceID:   "instance-test2",
			},
		},
	}

	gotEvent := NewRegistryEventFromDiff("registry-test1", oldSpecs, newSpecs)

	if !reflect.DeepEqual(wantEvent, gotEvent) {
		t.Fatalf("registry event:\nwant %+v\ngot %+v", wantEvent, gotEvent)
	}
}
