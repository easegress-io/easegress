package storage

type (
	// Informer is the dedicated informer for Mesh Controller.
	Informer interface {
		// TODO: handle Add/Update/Delete events.
	}

	informer struct {
		storage Storage
	}
)

// NewInformer creates an Informer.
func NewInformer(storage Storage) Informer {
	return &informer{
		storage: storage,
	}
}
