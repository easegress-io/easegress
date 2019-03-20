package ratelimiter

import (
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"golang.org/x/time/rate"
)

type (

	// Runtime contains runtime info of RateLimiter.
	Runtime struct {
		limiter *rate.Limiter
		rate1   metrics.EWMA
		done    chan struct{}
	}

	// Status contains status info of RateLimiter.
	Status struct {
		TPS uint64 `yaml:"tps"`
	}
)

// NewRuntime creates a RateLimiter runtime.
func NewRuntime() *Runtime {
	rate1 := metrics.NewEWMA1()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-time.After(5 * time.Second):
				rate1.Tick()
			case <-done:
				return
			}
		}
	}()

	return &Runtime{
		limiter: rate.NewLimiter(0, 1),
		rate1:   rate1,
		done:    done,
	}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return &Status{
		TPS: uint64(r.rate1.Rate()),
	}
}

// Close closes Runtime.
func (r *Runtime) Close() {
	close(r.done)
}
