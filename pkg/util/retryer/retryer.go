package retryer

import (
	"fmt"
	"math/rand"
	"time"
)

type (
	// BackOffPolicy defines back off policy
	BackOffPolicy uint8

	// Policy defines the policy of a retryer
	Policy struct {
		MaxAttempts         int
		WaitDuration        time.Duration
		BackOffPolicy       BackOffPolicy
		RandomizationFactor float64
	}

	// Event defines the event of a retry
	Event struct {
		Time    time.Time
		Attempt int
		Error   string
	}

	// EventListenerFunc is a listener function to listen retry event
	EventListenerFunc func(event *Event)

	// Retryer defines a retryer
	Retryer struct {
		policy   *Policy
		listener EventListenerFunc
	}
)

const (
	RandomBackOff BackOffPolicy = iota
	ExponentiallyBackOff
)

// NewPolicy create and initialize a policy with default configuration
func NewPolicy() *Policy {
	return &Policy{}
}

// New creates a retryer based on `policy`,
func New(policy *Policy) *Retryer {
	return &Retryer{policy: policy}
}

// SetStateListener sets a state listener for the Retryer
func (r *Retryer) SetStateListener(listener EventListenerFunc) {
	r.listener = listener
}

func (r *Retryer) notifyListener(attempt int, err string) {
	if r.listener != nil {
		event := Event{
			Time:    time.Now(),
			Attempt: attempt,
			Error:   err,
		}
		go r.listener(&event)
	}
}

// Execute executes the given function
func (r *Retryer) Execute(fn func() (interface{}, error)) (interface{}, error) {
	attempt := 0
	base := float64(r.policy.WaitDuration)
	for {
		attempt++
		result, e := fn()
		if e == nil {
			return result, e
		}

		r.notifyListener(attempt, e.Error())
		if attempt == r.policy.MaxAttempts {
			return nil, fmt.Errorf("failed after %d attempts, the last error is: %s", attempt, e.Error())
		}

		delta := base * r.policy.RandomizationFactor
		d := base - delta + float64(rand.Intn(int(delta*2+1)))
		time.Sleep(time.Duration(d))

		if r.policy.BackOffPolicy == ExponentiallyBackOff {
			base *= 1.5
		}
	}
}
