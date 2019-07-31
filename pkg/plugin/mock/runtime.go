package mock

type (

	// Runtime contains runtime info of Mock.
	Runtime struct{}

	// Status contains status info of Mock.
	Status struct{}
)

// NewRuntime creates a Mock runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return nil
}

// Close closes Runtime.
func (r *Runtime) Close() {}
