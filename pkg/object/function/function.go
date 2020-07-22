package function

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/plugin/backend"
	"github.com/megaease/easegateway/pkg/scheduler"
	"github.com/megaease/easegateway/pkg/util/httpstat"
	"github.com/megaease/easegateway/pkg/v"

	cron "github.com/robfig/cron/v3"
	yaml "gopkg.in/yaml.v2"
)

const (
	// Kind is Function kind.
	Kind = "Function"
)

func init() {
	scheduler.Register(&scheduler.ObjectRecord{
		Kind:              Kind,
		DefaultSpecFunc:   DefaultSpec,
		NewFunc:           New,
		DependObjectKinds: []string{httpserver.Kind},
	})
}

type (
	// Function is Object Function.
	Function struct {
		spec *Spec

		handlers *sync.Map

		backend *backend.Backend
		cron    *Cron
	}

	// Spec describes the Function.
	Spec struct {
		scheduler.ObjectMeta `yaml:",inline"`

		URL  string    `yaml:"url" jsonschema:"required"`
		Cron *CronSpec `yaml:"cron" jsonschema:"omitempty"`
	}

	// Status is the status of Function.
	Status struct {
		Timestamp int64  `yaml:"timestamp"`
		Health    string `yaml:"health"`

		HTTP *httpstat.Status `yaml:"http"`
		Cron *CronStatus      `yaml:"cron"`
	}
)

const (
	// withoutSecondOpt is the standard cron format of unix.
	withoutSecondOpt = cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor
	withSecondOpt    = cron.Second | withoutSecondOpt
	// optionalSecondOpt is not used for now.
	optionalSecondOpt = cron.SecondOptional | withSecondOpt
)

// Validate validates Spec.
func (spec Spec) Validate() error {
	backendSpec := spec.backendSpec()
	buff, err := yaml.Marshal(backendSpec)
	if err != nil {
		err = fmt.Errorf("BUG: marshal %#v to yaml failed: %v",
			backendSpec, err)
		logger.Errorf(err.Error())
		return err
	}

	vr := v.Validate(backendSpec, buff)
	if !vr.Valid() {
		return fmt.Errorf("%s", vr.Error())
	}

	return nil
}

func (spec Spec) backendSpec() *backend.Spec {
	return &backend.Spec{
		MainPool: &backend.PoolSpec{
			Servers: []*backend.Server{
				{
					URL: spec.URL,
				},
			},
			LoadBalance: &backend.LoadBalance{Policy: backend.PolicyRoundRobin},
		},
	}
}

// New creates an Function.
func New(spec *Spec, prev *Function, handlers *sync.Map) *Function {
	var prevBackend *backend.Backend
	if prev != nil {
		prevBackend = prev.backend
	}

	f := &Function{
		spec:     spec,
		handlers: handlers,
		backend:  backend.New(spec.backendSpec(), prevBackend),
	}

	if spec.Cron != nil {
		f.cron = NewCron(spec.URL, spec.Cron)
	}

	f.handlers.Store(spec.Name, f)

	return f
}

// DefaultSpec returns Function default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

// Handle handles all HTTP incoming traffic.
func (f *Function) Handle(ctx context.HTTPContext) {
	f.backend.Handle(ctx)
}

// Status returns Status genreated by Runtime.
func (f *Function) Status() *Status {
	s := &Status{
		HTTP: f.backend.Status().MainPool.Stat,
	}

	if f.cron != nil {
		s.Cron = f.cron.Status()
	}

	return s
}

// Close closes Function.
func (f *Function) Close() {
	if f.cron != nil {
		f.cron.Close()
	}

	f.handlers.Delete(f.spec.Name)
	f.backend.Close()
}
