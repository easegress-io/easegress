package compression

type (

	// Runtime contains runtime info of Compression.
	Runtime struct{}

	// Status contains status info of Compression.
	Status struct{}
)

// NewRuntime creates a Compression runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return nil
}

// Close closes Runtime.
func (r *Runtime) Close() {}
