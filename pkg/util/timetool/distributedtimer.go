package timetool

import (
	"time"
)

type (
	// NextDurationFunc is the type for registering callback function
	// to determinate the timer to tick after the returned duration.
	NextDurationFunc func() time.Duration

	// DistributedTimer keeps timer's tick time consistent.
	// Please notice nextDurationFunc assumes different instances
	// running under the same system clock.
	DistributedTimer struct {
		nextDurationFunc NextDurationFunc

		C chan time.Time

		done chan struct{}
	}
)

// NewDistributedTimer creates a DistributedTimer
func NewDistributedTimer(nextDurationFunc NextDurationFunc) *DistributedTimer {
	dt := &DistributedTimer{
		nextDurationFunc: nextDurationFunc,
		C:                make(chan time.Time),
		done:             make(chan struct{}),
	}

	go dt.run()

	return dt
}

func (dt *DistributedTimer) run() {
	for {
		select {
		case <-dt.done:
			return
		case now := <-time.After(dt.nextDurationFunc()):
			dt.C <- now
		}
	}
}

// Close closes DistributedTimer.
func (dt *DistributedTimer) Close() {
	close(dt.done)
}
