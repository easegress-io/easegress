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

package shadowservice

import (
	"fmt"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/supervisor"
)

type (
	// ShadowService is the business layer between mesh and store.
	// It is not concurrently safe, the users need to do it by themselves.
	ShadowService struct {
		superSpec *supervisor.Spec
		spec      *spec.Admin

		store storage.Storage
	}
)

// New creates a service with spec
func New(superSpec *supervisor.Spec) *ShadowService {
	s := &ShadowService{
		superSpec: superSpec,
		spec:      superSpec.ObjectSpec().(*spec.Admin),
		store:     storage.New(superSpec.Name(), superSpec.Super().Cluster()),
	}

	return s
}

// PutSpec writes the shadow service spec
func (ss *ShadowService) PutSpec(spec *spec.ShadowService) {
	buff, err := yaml.Marshal(spec)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", spec, err))
	}

	err = ss.store.Put(layout.ShadowServiceSpecKey(spec.Name), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

// GetSpec gets the shadow service spec by its name
func (ss *ShadowService) GetSpec(name string) *spec.ShadowService {
	spec, _ := ss.GetSpecWithInfo(name)
	return spec
}

// GetSpecWithInfo gets the shadow service spec by its name
func (ss *ShadowService) GetSpecWithInfo(name string) (*spec.ShadowService, *mvccpb.KeyValue) {
	kv, err := ss.store.GetRaw(layout.ShadowServiceSpecKey(name))
	if err != nil {
		api.ClusterPanic(err)
	}

	if kv == nil {
		return nil, nil
	}

	spec := &spec.ShadowService{}
	err = yaml.Unmarshal([]byte(kv.Value), spec)
	if err != nil {
		panic(fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", string(kv.Value), err))
	}

	return spec, kv
}

// DeleteSpec deletes shadow service spec by its name
func (ss *ShadowService) DeleteSpec(name string) {
	err := ss.store.Delete(layout.ShadowServiceSpecKey(name))
	if err != nil {
		api.ClusterPanic(err)
	}
}

// ListSpecs lists shadow services specs
func (ss *ShadowService) ListSpecs() []*spec.ShadowService {
	specs := []*spec.ShadowService{}
	kvs, err := ss.store.GetPrefix(layout.ShadowServiceSpecPrefix())
	if err != nil {
		api.ClusterPanic(err)
	}

	for _, v := range kvs {
		spec := &spec.ShadowService{}
		err := yaml.Unmarshal([]byte(v), spec)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		specs = append(specs, spec)
	}

	return specs
}

// DeleteSpecsOfService delete all shadow services of the specified service
func (ss *ShadowService) DeleteSpecsOfService(serviceName string) {
	kvs, err := ss.store.GetPrefix(layout.ShadowServiceSpecPrefix())
	if err != nil {
		api.ClusterPanic(err)
	}

	for _, v := range kvs {
		spec := &spec.ShadowService{}
		err := yaml.Unmarshal([]byte(v), spec)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		if spec.ServiceName == serviceName {
			ss.DeleteSpec(spec.Name)
		}
	}
}
