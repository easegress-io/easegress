package candidatebackend

type (

	// Runtime contains runtime info of CandidateBackend.
	Runtime struct {
		cb *CandidateBackend
	}

	// Status contains status info of CandidateBackend.
	Status struct {
		Codes map[string]map[int]uint64 `yaml:"codes"`
	}
)

// NewRuntime creates a CandidateBackend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return &Status{
		Codes: r.cb.backend.Codes(),
	}
}

// Close closes Runtime.
func (r *Runtime) Close() {}
