package fallback

type (

	// Runtime contains runtime info of Fallback.
	Runtime struct{}

	// Status contains status info of Fallback.
	Status struct{}
)

// NewRuntime creates a Fallback runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return nil
}

// Close closes Runtime.
func (r *Runtime) Close() {}
