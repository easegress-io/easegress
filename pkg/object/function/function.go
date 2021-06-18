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

package function

import (
	"fmt"

	cron "github.com/robfig/cron/v3"
	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filter/proxy"
	"github.com/megaease/easegress/pkg/filter/requestadaptor"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/httpstat"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
	"github.com/megaease/easegress/pkg/v"
)

const (
	// Category is the category of Function.
	Category = supervisor.CategoryPipeline

	// Kind is the kind of Function.
	Kind = "Function"

	// withoutSecondOpt is the standard cron format of unix.
	withoutSecondOpt = cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor
	withSecondOpt    = cron.Second | withoutSecondOpt
	// optionalSecondOpt is not used for now.
	optionalSecondOpt = cron.SecondOptional | withSecondOpt
)

func init() {
	supervisor.Register(&Function{})
}

type (
	// Function is Object Function.
	Function struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		proxy          *proxy.Proxy
		cron           *Cron
		requestAdaptor *requestadaptor.RequestAdaptor
	}

	// Spec describes the Function.
	Spec struct {
		URL            string               `yaml:"url" jsonschema:"required"`
		Cron           *CronSpec            `yaml:"cron" jsonschema:"omitempty"`
		RequestAdaptor *RequestAdapotorSpec `yaml:"requestAdaptor" jsonschema:"omitempty"`
	}

	// RequestAdapotorSpec describes the RequestAdaptor.
	RequestAdapotorSpec struct {
		Method string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path   *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
	}

	// Status is the status of Function.
	Status struct {
		Health string `yaml:"health"`

		HTTP *httpstat.Status `yaml:"http"`
		Cron *CronStatus      `yaml:"cron"`
	}
)

// Validate validates Spec.
func (spec Spec) Validate() error {
	pipeSpec := spec.proxyPipeSpec()
	buff, err := yaml.Marshal(pipeSpec)
	if err != nil {
		err = fmt.Errorf("BUG: marshal %#v to yaml failed: %v",
			pipeSpec, err)
		logger.Errorf(err.Error())
		return err
	}

	vr := v.Validate(pipeSpec, buff)
	if !vr.Valid() {
		return fmt.Errorf("%s", vr.Error())
	}

	return nil
}

func (spec Spec) proxyPipeSpec() *httppipeline.FilterSpec {
	meta := &httppipeline.FilterMetaSpec{
		Kind: proxy.Kind,
		Name: "proxy",
	}
	filterSpec := &proxy.Spec{
		MainPool: &proxy.PoolSpec{
			Servers: []*proxy.Server{
				{
					URL: spec.URL,
				},
			},
			LoadBalance: &proxy.LoadBalance{Policy: proxy.PolicyRoundRobin},
		},
	}

	pipeSpec, err := httppipeline.NewFilterSpec(meta, filterSpec)
	if err != nil {
		panic(err)
	}

	return pipeSpec
}

func (spec Spec) requestAdaptorPipeSpec() *httppipeline.FilterSpec {
	meta := &httppipeline.FilterMetaSpec{
		Kind: requestadaptor.Kind,
		Name: "urlratelimiter",
	}
	filterSpec := &requestadaptor.Spec{
		Method: spec.RequestAdaptor.Method,
		Path:   spec.RequestAdaptor.Path,
		Header: spec.RequestAdaptor.Header,
	}

	pipeSpec, err := httppipeline.NewFilterSpec(meta, filterSpec)
	if err != nil {
		panic(err)
	}

	return pipeSpec
}

// Category returns the category of Function.
func (f *Function) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of Function.
func (f *Function) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Function.
func (f *Function) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes Function.
func (f *Function) Init(superSpec *supervisor.Spec,
	super *supervisor.Supervisor, muxMapper protocol.MuxMapper) {

	f.superSpec, f.spec, f.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	f.reload()
}

// Inherit inherits previous generation of Function.
func (f *Function) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object,
	super *supervisor.Supervisor, muxMapper protocol.MuxMapper) {

	previousGeneration.Close()
	f.Init(superSpec, super, muxMapper)
}

func (f *Function) reload() {
	f.proxy = &proxy.Proxy{}
	f.proxy.Init(f.spec.proxyPipeSpec(), f.super)

	if f.spec.RequestAdaptor != nil {
		f.requestAdaptor = &requestadaptor.RequestAdaptor{}
		f.requestAdaptor.Init(f.spec.requestAdaptorPipeSpec(), f.super)
	}

	if f.spec.Cron != nil {
		f.cron = NewCron(f.spec.URL, f.spec.Cron)
	}
}

// Handle handles all HTTP incoming traffic.
func (f *Function) Handle(ctx context.HTTPContext) {
	if f.requestAdaptor != nil {
		f.requestAdaptor.Handle(ctx)
	}
	f.proxy.Handle(ctx)
}

// Status returns Status genreated by Runtime.
func (f *Function) Status() *supervisor.Status {
	s := &Status{
		HTTP: f.proxy.Status().(*proxy.Status).MainPool.Stat,
	}

	if f.cron != nil {
		s.Cron = f.cron.Status()
	}

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes Function.
func (f *Function) Close() {
	if f.requestAdaptor != nil {
		f.requestAdaptor.Close()
	}

	if f.cron != nil {
		f.cron.Close()
	}

	f.proxy.Close()
}
