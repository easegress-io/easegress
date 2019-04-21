package candidatebackend

type (

	// Runtime contains runtime info of CandidateBackend.
	Runtime struct{}

	// Status contains status info of CandidateBackend.
	Status struct{}
)

// NewRuntime creates a CandidateBackend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return nil
}

// Close closes Runtime.
func (r *Runtime) Close() {}
