package circuitbreaker

type (

	// Runtime contains runtime info of CircuitBreaker.
	Runtime struct {
		cb *CircuitBreaker
	}

	// Status contains status info of CircuitBreaker.
	Status struct {
		State state `yaml:"state"`
	}
)

// NewRuntime creates a CircuitBreaker runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	if r.cb == nil {
		return nil
	}

	return &Status{
		State: r.cb.state(),
	}
}

// Close closes Runtime.
func (r *Runtime) Close() {}
