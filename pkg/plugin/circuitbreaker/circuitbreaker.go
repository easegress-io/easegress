package circuitbreaker

import (
	"fmt"
	"time"

	"github.com/megaease/easegateway/pkg/context"

	"github.com/sony/gobreaker"
)

const (
	stateClosed   state = "closed"
	stateHalfOpen       = "halfOpen"
	stateOpen           = "open"
)

type (
	state string

	// CircuitBreaker is plugin CircuitBreaker.
	CircuitBreaker struct {
		spec *Spec
		cb   *gobreaker.CircuitBreaker
	}

	// Spec describes the CircuitBreaker.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		FailureCodes                   []int   `yaml:"failureCodes" v:"gte=1,unique,dive,httpcode"`
		CountPeriodSeconds             uint16  `yaml:"countPeriodSeconds" v:"gte=1"`
		ToClosedConsecutiveCounts      uint32  `yaml:"toClosedConsecutiveCounts" v:"gte=1"`
		ToHalfOpenSeconds              uint16  `yaml:"toHalfOpenSeconds" v:"gte=1"`
		ToOpenFailureCounts            *uint32 `yaml:"toOpenFailureCounts" v:"omitempty,gte=1"`
		ToOpenFailureConsecutiveCounts *uint32 `yaml:"toOpenFailureConsecutiveCounts" v:"omitempty,gte=1"`
	}
)

// Validate validates Spec.
func (s Spec) Validate() error {
	if s.ToOpenFailureCounts == nil &&
		s.ToOpenFailureConsecutiveCounts == nil {
		return fmt.Errorf("toOpenFailureCounts," +
			"toOpenFailureConsecutiveCounts " +
			"are all empty")

	}

	return nil
}

// New creates a CircuitBreaker.
func New(spec *Spec, runtime *Runtime) *CircuitBreaker {
	st := gobreaker.Settings{
		MaxRequests: spec.ToClosedConsecutiveCounts,
		Interval:    time.Duration(spec.CountPeriodSeconds) * time.Second,
		Timeout:     time.Duration(spec.ToHalfOpenSeconds) * time.Second,
	}
	st.ReadyToTrip = func(counts gobreaker.Counts) bool {
		if spec.ToOpenFailureCounts != nil &&
			*spec.ToOpenFailureCounts <= counts.TotalFailures {
			return true
		}
		if spec.ToOpenFailureConsecutiveCounts != nil &&
			*spec.ToOpenFailureConsecutiveCounts <= counts.ConsecutiveFailures {
			return true
		}

		return false
	}

	cb := &CircuitBreaker{
		spec: spec,
		cb:   gobreaker.NewCircuitBreaker(st),
	}

	runtime.cb = cb

	return cb
}

// Close closes CircuitBreaker.
// Nothing to do.
func (cb *CircuitBreaker) Close() {}

// Protect protects Handler.
func (cb *CircuitBreaker) Protect(ctx context.HTTPContext, handler func(ctx context.HTTPContext)) error {
	failureCode := -1
	_, err := cb.cb.Execute(func() (interface{}, error) {
		handler(ctx)

		code := ctx.Response().StatusCode()
		for _, fc := range cb.spec.FailureCodes {
			if fc == code {
				failureCode = code
				// NOTICE: The error is never used, just show it in here.
				return nil, fmt.Errorf("failureCode: %d", code)
			}
		}

		return nil, nil
	})

	// NOTICE: Just count the failure code, the CircuitBreaker is not open yet.
	if failureCode != -1 {
		return nil
	}

	return err
}

func (cb *CircuitBreaker) state() state {
	switch cb.cb.State() {
	case gobreaker.StateClosed:
		return stateClosed
	case gobreaker.StateHalfOpen:
		return stateHalfOpen
	case gobreaker.StateOpen:
		return stateOpen
	}

	// Never be here.
	return "unknownState"
}
