package codecounter

// CodeCounter is the gouroutine unsafe code counter.
type CodeCounter struct {
	//      code:count
	counter map[int]uint64
}

// New creates a CodeCounter.
func New() *CodeCounter {
	return &CodeCounter{
		counter: make(map[int]uint64),
	}
}

// Count counts a new code.
func (cc *CodeCounter) Count(code int) {
	cc.counter[code]++
}

// Codes returns the codes.
func (cc *CodeCounter) Codes() map[int]uint64 {
	codes := make(map[int]uint64)
	for code, count := range cc.counter {
		codes[code] = count
	}

	return codes
}
