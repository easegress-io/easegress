package backend

type (

	// Runtime contains runtime info of Backend.
	Runtime struct {
		b *Backend
	}

	// Status contains status info of Backend.
	Status struct {
		Codes map[string]map[int]uint64 `yaml:"codes"`
	}
)

// NewRuntime creates a Backend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return &Status{
		Codes: r.b.backend.Codes(),
	}
}

// Close closes Runtime.
func (r *Runtime) Close() {}
