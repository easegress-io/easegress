package scheduler

import (
	"time"
)

const (
	syncStatusPaceInUnixSeconds = 5
)

func nextSyncStatusDuration() time.Duration {
	return nextDuration(time.Now(), syncStatusPaceInUnixSeconds)
}

// nextDuration returns the next duration after t,
// plus satisfying the pace in UNIX seconds(minimum unit for now).
func nextDuration(t time.Time, paceInUnixSeconds int) time.Duration {
	paceInNano := int64(paceInUnixSeconds) * int64(time.Second/time.Nanosecond)
	rounds := t.UnixNano() / paceInNano

	return time.Duration(paceInNano*(rounds+1) - t.UnixNano())
}
