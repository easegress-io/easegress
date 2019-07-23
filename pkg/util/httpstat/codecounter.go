package httpstat

type codeCounter struct {
	//      code:count
	counter map[int]uint64
}

func newCodeCounter() *codeCounter {
	return &codeCounter{
		counter: make(map[int]uint64),
	}
}

func (cc *codeCounter) count(code int) {
	cc.counter[code]++
}

func (cc *codeCounter) codes() map[int]uint64 {
	codes := make(map[int]uint64)
	for code, count := range cc.counter {
		codes[code] = count
	}

	return codes
}
