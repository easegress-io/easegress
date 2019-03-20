package validator

type (
	// Runtime contains runtime info of Validator.
	Runtime struct{}

	// Status contains status info of Validator.
	Status struct{}
)

// NewRuntime creates a Validator runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return nil
}

// Close closes Runtime.
func (r *Runtime) Close() {}
