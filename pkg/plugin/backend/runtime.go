package backend

type (

	// Runtime contains runtime info of Backend.
	Runtime struct{}

	// Status contains status info of Backend.
	Status struct{}
)

// NewRuntime creates a Backend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return nil
}

// Close closes Runtime.
func (r *Runtime) Close() {}
