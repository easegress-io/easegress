package function

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/filter/backend"
	"github.com/megaease/easegateway/pkg/filter/requestadaptor"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/httpstat"
	"github.com/megaease/easegateway/pkg/util/pathadaptor"
	"github.com/megaease/easegateway/pkg/v"

	cron "github.com/robfig/cron/v3"
	yaml "gopkg.in/yaml.v2"
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
		spec *Spec

		backend        *backend.Backend
		cron           *Cron
		requestAdaptor *requestadaptor.RequestAdaptor
	}

	// Spec describes the Function.
	Spec struct {
		supervisor.ObjectMetaSpec `yaml:",inline"`

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

func (spec Spec) requestAdaptorSpec() *requestadaptor.Spec {
	if spec.RequestAdaptor == nil {
		return &requestadaptor.Spec{}
	}

	return &requestadaptor.Spec{
		Method: spec.RequestAdaptor.Method,
		Path:   spec.RequestAdaptor.Path,
		Header: spec.RequestAdaptor.Header,
	}
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
func (f *Function) DefaultSpec() supervisor.ObjectSpec {
	return &Spec{}
}

// Renew renews Function.
func (f *Function) Renew(spec supervisor.ObjectSpec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	var prevBackend *backend.Backend
	if previousGeneration != nil {
		prevBackend = previousGeneration.(*Function).backend
	}

	var prevRequestAdaptor *requestadaptor.RequestAdaptor
	if previousGeneration != nil {
		prevRequestAdaptor = previousGeneration.(*Function).requestAdaptor
	}

	f.spec = spec.(*Spec)
	f.backend = backend.New(f.spec.backendSpec(), prevBackend)

	if f.spec.RequestAdaptor != nil {
		f.requestAdaptor = requestadaptor.New(f.spec.requestAdaptorSpec(), prevRequestAdaptor)
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
	f.backend.Handle(ctx)
}

// Status returns Status genreated by Runtime.
func (f *Function) Status() interface{} {
	s := &Status{}

	if f.spec == nil {
		return s
	}

	s.HTTP = f.backend.Status().MainPool.Stat

	if f.cron != nil {
		s.Cron = f.cron.Status()
	}

	return s
}

// Close closes Function.
func (f *Function) Close() {
	if f.requestAdaptor != nil {
		f.requestAdaptor.Close()
	}

	if f.cron != nil {
		f.cron.Close()
	}

	f.backend.Close()
}
