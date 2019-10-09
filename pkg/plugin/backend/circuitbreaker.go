package backend

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/stringtool"

	"github.com/sony/gobreaker"
)

const (
	stateClosed   = "closed"
	stateHalfOpen = "halfOpen"
	stateOpen     = "open"
)

type (
	circuitBreaker struct {
		spec *circuitBreakerSpec
		cb   *gobreaker.CircuitBreaker
	}

	circuitBreakerSpec struct {
		V string `yaml:"-" v:"parent"`

		CountPeriod                    string  `yaml:"countPeriod" v:"required,duration,dmin=1s"`
		ToClosedConsecutiveCounts      uint32  `yaml:"toClosedConsecutiveCounts" v:"gte=1"`
		ToHalfOpenTimeout              string  `yaml:"toHalfOpenTimeout" v:"required,duration,dmin=1s"`
		ToOpenFailureCounts            *uint32 `yaml:"toOpenFailureCounts" v:"omitempty,gte=1"`
		ToOpenFailureConsecutiveCounts *uint32 `yaml:"toOpenFailureConsecutiveCounts" v:"omitempty,gte=1"`

		failureCodes []int
	}
)

func (s circuitBreakerSpec) Validate() error {
	if s.ToOpenFailureCounts == nil &&
		s.ToOpenFailureConsecutiveCounts == nil {
		return fmt.Errorf("toOpenFailureCounts," +
			"toOpenFailureConsecutiveCounts " +
			"are all empty")

	}

	return nil
}

// newCircuitBreaker creates a CircuitBreaker.
func newCircuitBreaker(spec *circuitBreakerSpec, failureCodes []int) *circuitBreaker {
	spec.failureCodes = failureCodes

	interval, err := time.ParseDuration(spec.CountPeriod)
	if err != nil {
		logger.Errorf("BUG: parse CountPeriod %s to duration failed: %v",
			spec.CountPeriod, err)
	}
	timeout, err := time.ParseDuration(spec.ToHalfOpenTimeout)
	if err != nil {
		logger.Errorf("BUG: parse ToHalfOpenTimeout %s to duration failed: %v",
			spec.CountPeriod, err)
	}

	st := gobreaker.Settings{
		MaxRequests: spec.ToClosedConsecutiveCounts,
		Interval:    interval,
		Timeout:     timeout,
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

	cb := &circuitBreaker{
		spec: spec,
		cb:   gobreaker.NewCircuitBreaker(st),
	}

	return cb
}

// Protect protects Handler.
func (cb *circuitBreaker) protect(ctx context.HTTPContext, reqBody io.Reader,
	handler func(ctx context.HTTPContext, reqBody io.Reader)) error {
	handled := false
	_, err := cb.cb.Execute(func() (interface{}, error) {
		handled = true
		handler(ctx, reqBody)

		code := ctx.Response().StatusCode()
		for _, fc := range cb.spec.failureCodes {
			if fc == code {
				// NOTE: The error is never used, just show it in here.
				return nil, fmt.Errorf("failureCode: %d", code)
			}
		}

		return nil, nil
	})

	// Only for opening circuitBreaker.
	if err != nil && !handled {
		ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
		ctx.AddTag(stringtool.Cat("circuitBreaker: ", err.Error()))
		return err
	}

	return nil
}

func (cb *circuitBreaker) status() string {
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
