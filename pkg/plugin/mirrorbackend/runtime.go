package mirrorbackend

type (

	// Runtime contains runtime info of MirrorBackend.
	Runtime struct{}

	// Status contains status info of MirrorBackend.
	Status struct{}
)

// NewRuntime creates a MirrorBackend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return nil
}

// Close closes Runtime.
func (r *Runtime) Close() {}
