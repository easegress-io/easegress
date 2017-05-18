package cluster

import (
	"sync/atomic"
)

type logicalTime uint64

// Implementation of https://en.wikipedia.org/wiki/Lamport_timestamps

type logicalClock struct {
	time uint64
}

// Call this on sending a message
func (lc *logicalClock) Increase() logicalTime {
	return logicalTime(atomic.AddUint64(&lc.time, 1))
}

// Call this on receiving a message
func (lc *logicalClock) Update(time logicalTime) logicalTime {
	for {
		localTime := atomic.LoadUint64(&lc.time)
		otherTime := uint64(time)
		if otherTime < localTime {
			// Nothing to do.
			break
		}

		if atomic.CompareAndSwapUint64(&lc.time, localTime, otherTime+1) {
			break
		}
	}

	return logicalTime(atomic.LoadUint64(&lc.time))
}

func (lc *logicalClock) Time() logicalTime {
	return logicalTime(atomic.LoadUint64(&lc.time))
}

const (
	zeroLogicalTime = logicalTime(0)
)
