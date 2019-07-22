package seckill

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"time"

	"github.com/megaease/easegateway/pkg/object/httpproxy"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/registry"
)

func init() {
	registry.Register(Kind, DefaultSpec)
}

const (
	// Kind is Seckill kind.
	Kind = "Seckill"
)

type (
	// Seckill is Object HTTPProxy.
	Seckill struct {
		spec *Spec

		runtime *Runtime

		pathSecret string
	}

	// Spec describes the HTTPProxy.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		registry.MetaSpec `yaml:",inline"`

		SceneID             string     `yaml:"sceneID" v:"required"`
		StartTime           string     `yaml:"startTime" v:"required,timerfc3339"`
		EndTime             string     `yaml:"endTime" v:"required,timerfc3339"`
		Redis               *RedisSpec `yaml:"redis" v:"required"`
		DoubleClickInterval string     `yaml:"doubleClickinterval" v:"required,duration,dmin=2s"`
		TimeCheckInterval   string     `yaml:"timeCheckInterval" v:"required,duration,dmin=10s"`
		StockCount          uint64     `yaml:"stockCount" v:"gt=0"`
		Proxy               string     `yaml:"proxy" v:"required"`

		startTime time.Time
		endTime   time.Time
	}

	// RedisSpec describes redis client config.
	RedisSpec struct {
		Addr     string `yaml:"addr" v:"required,hostport"`
		Password string `yaml:"password" v:"omitempty"`
		DB       int    `yaml:"db" v:"gte=0"`
	}
)

// Validate validates Spec.
func (s Spec) Validate() error {
	startTime, err := time.Parse(time.RFC3339, s.StartTime)
	if err != nil {
		return fmt.Errorf("parse startTime failed: %v", err)
	}
	endTime, err := time.Parse(time.RFC3339, s.EndTime)
	if err != nil {
		return fmt.Errorf("parse endTime failed: %v", err)
	}
	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	// FIXME: Maybe the gap needs to be larger than 1 hour?
	if startTime.Before(now) {
		return fmt.Errorf("startTime %s before the server time %s",
			startTime, nowStr)
	}
	// FIXME: Maybe the gap need to be larger than 5 minutes?
	if endTime.Before(startTime) {
		return fmt.Errorf("endTime %s before the startTime %s",
			endTime, startTime)
	}

	if s.Name == s.Proxy {
		return fmt.Errorf("can't set proxy with self %s", s.Name)
	}

	return nil
}

// New creates an Seckill.
func New(spec *Spec, runtime *Runtime) *Seckill {
	runtime.reload(spec)

	startTime, err := time.Parse(time.RFC3339, spec.StartTime)
	if err != nil {
		logger.Errorf("BUG: parse startTime %s failed: %v", spec.StartTime, err)
	}
	endTime, err := time.Parse(time.RFC3339, spec.EndTime)
	if err != nil {
		logger.Errorf("BUG: parse endTime %s failed: %v", spec.EndTime, err)
	}
	spec.startTime, spec.endTime = startTime, endTime

	// NOTE: Use hashing SceneID to guarantee
	// that every member generates the same value.
	hash := fnv.New32a()
	hash.Write([]byte(spec.SceneID))
	hash.Sum32()
	pathSecret := fmt.Sprintf("%d", hash.Sum32())

	s := &Seckill{
		spec:       spec,
		runtime:    runtime,
		pathSecret: pathSecret,
	}

	return s
}

// DefaultSpec returns Seckill default spec.
func DefaultSpec() registry.Spec {
	return &Spec{
		TimeCheckInterval:   "30s",
		DoubleClickInterval: "2s",
	}
}

// Handle handles all incoming traffic.
func (s *Seckill) Handle(ctx context.HTTPContext) {
	if time.Now().Before(s.spec.startTime) {
		s.countdown(ctx)
		return
	}

	w := ctx.Response()

	secretOK := s.checkPathSecret(ctx)
	if !secretOK {
		w.SetStatusCode(http.StatusNotFound)
		return
	}

	doubleClickOK := s.checkDoubleClick(ctx)
	if !doubleClickOK {
		w.SetStatusCode(http.StatusTooManyRequests)
		return
	}

	passportOK := s.checkPassport(ctx)
	if !passportOK {
		luckOK := s.checkLuck(ctx)
		if !luckOK {
			// TODO: Set status code which means that you are refused by probability
			return
		}

		qualificationOK := s.checkQualification(ctx)
		if !qualificationOK {
			// TODO: Set status code which means that you are not quilified(blacklist/whitelist).
			return
		}

		passportIssued := s.issuePassport(ctx)
		if !passportIssued {
			// TODO: Set status code which means that stock is empty.
			return
		}
	}

	s.sendToProxy(ctx)
}

func (s *Seckill) checkPathSecret(ctx context.HTTPContext) bool {
	return false
}

// countdown returns start_time and next_time before the Seckill starts.
// the start_time and next_time both are relative time in milliseconds.
// start_time means how long the Seckill starts from now on.
// next_time means how long the client asks me again from now on.
// real_uuid is empty until the Seckil starts, then it will become s.pathSecret.
// FIXME: if the client has not be developed, we propose `real_uuid` changing to `path_secret`.
// {
// 	“start_time”: “3000000”,
// 	“next_time”: “9000”,
// 	“real_uuid”: “uuid-generated-by-gateway”
// }
func (s *Seckill) countdown(ctx context.HTTPContext) {

}

func (s *Seckill) checkDoubleClick(ctx context.HTTPContext) bool {
	return false
}

func (s *Seckill) checkPassport(ctx context.HTTPContext) bool {
	return false
}

func (s *Seckill) issuePassport(ctx context.HTTPContext) bool {
	// TODO: Rewrite Set-Cookie
	return false
}

func (s *Seckill) checkLuck(ctx context.HTTPContext) bool {
	return false
}

func (s *Seckill) checkQualification(ctx context.HTTPContext) bool {
	return false
}

func (s *Seckill) sendToProxy(ctx context.HTTPContext) {
	handler, exists := s.runtime.handlers.Load(s.spec.Proxy)
	if !exists {
		ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
		ctx.AddTag(fmt.Sprintf("proxy %s not found", s.spec.Proxy))
		return
	}
	proxyHandler, ok := handler.(*httpproxy.HTTPProxy)
	if !ok {
		ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
		// NOTE: In case of another Seckill causing infinite recursion.
		ctx.AddTag(fmt.Sprintf("proxy %s want *httpproxy.HTTPPRoxy, got %T",
			s.spec.Proxy, handler))
		return
	}
	proxyHandler.Handle(ctx)
}

// Close closes Seckill.
func (s *Seckill) Close() {
}
