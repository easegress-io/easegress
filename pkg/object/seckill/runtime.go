package seckill

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/go-redis/redis"
	"github.com/megaease/easegateway/pkg/logger"
)

const (
	stateNil     stateType = "nil"
	stateFailed            = "failed"
	stateLoading           = "loading" // load redis data
	stateReady             = "ready"
	stateClosed            = "closed"
)

var (
	errNil = fmt.Errorf("")
)

type (
	stateType string

	// Runtime contains all runtime info of Seckill.
	Runtime struct {
		spec        *Spec
		handlers    *sync.Map
		redisClient *redis.Client

		// status
		state atomic.Value // stateType
		err   atomic.Value // error
	}

	// Status contains all status gernerated by runtime, for displaying to users.
	Status struct {
		Timestamp uint64 `yaml:"timestamp"`

		Error string `yaml:"error,omitempty"`
	}
)

// InjectTimestamp injects timestamp.
func (s *Status) InjectTimestamp(t uint64) { s.Timestamp = t }

// NewRuntime creates an Seckill runtime.
func NewRuntime(handlers *sync.Map) *Runtime {
	return &Runtime{
		handlers: handlers,
	}
}

func (r *Runtime) reload(spec *Spec) {
	if r.spec == nil || !reflect.DeepEqual(r.spec.Redis, spec.Redis) {
		if r.redisClient != nil {
			err := r.redisClient.Close()
			if err != nil {
				logger.Errorf("close redis client failed: %v", err)
			}
		}

		r.redisClient = redis.NewClient(
			&redis.Options{
				Addr:     spec.Redis.Addr,
				Password: spec.Redis.Password,
				DB:       spec.Redis.DB,
			},
		)
	}

	r.spec = spec

	r.setState(stateNil)
	r.setError(errNil)

	// TODO:
	// Load readis data for the every first time or
	// the spec.SceneID has changed.
	// Provide goroutine-safe methods wrapping reids data for business.
	// Please report the error by calling r.setError()
}

func (r *Runtime) setState(state stateType) {
	r.state.Store(state)
}

func (r *Runtime) getState() stateType {
	return r.state.Load().(stateType)
}

// Status returns Seckill Status.
func (r *Runtime) Status() *Status {
	return &Status{
		Error: r.getError().Error(),
	}
}

func (r *Runtime) setError(err error) {
	if err == nil {
		r.err.Store(errNil)
	} else {
		// NOTE: For type safe.
		r.err.Store(fmt.Errorf("%v", err))
	}
}

func (r *Runtime) getError() error {
	err := r.err.Load()
	if err == nil {
		return nil
	}
	return err.(error)
}

// Close closes runtime.
func (r *Runtime) Close() {
	err := r.redisClient.Close()
	if err != nil {
		logger.Errorf("close redis client failed: %v", err)
	}
}
