package adaptor

type (

	// Runtime contains runtime info of Adaptor.
	Runtime struct{}

	// Status contains status info of Adaptor.
	Status struct{}
)

// NewRuntime creates an Adaptor runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return nil
}

// Close closes Runtime.
func (r *Runtime) Close() {}
