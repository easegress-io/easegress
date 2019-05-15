package mirrorbackend

type (

	// Runtime contains runtime info of MirrorBackend.
	Runtime struct {
		mb *MirrorBackend
	}

	// Status contains status info of MirrorBackend.
	Status struct {
		Codes map[string]map[int]uint64 `yaml:"codes"`
	}
)

// NewRuntime creates a MirrorBackend runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Status returns status.
func (r *Runtime) Status() *Status {
	return &Status{
		Codes: r.mb.backend.Codes(),
	}
}

// Close closes Runtime.
func (r *Runtime) Close() {}
