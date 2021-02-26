package meshcontroller

import (
	"time"

	"github.com/megaease/easegateway/pkg/logger"
)

type Master struct {
	mss   *MeshServiceServer
	store MeshStorage

	ServiceWatchInterval string

	done chan struct{}
}

// NewMaster return a ini
func NewMaster() *Master {

	return &Master{}
}

func (m *Master) watchServicesHeartbeat() {

	// Get all serivces
	m.mss.WatchAllSerivceInstanceHeartbeat()
	// find one serivce instance

	// read heartbeat, if more than 30s (configurable), then set the instance to OUT_OF_SERVICE

	return
}

// Run is the entry of the Master role controller
func (m *Master) Run() {
	watchInterval, err := time.ParseDuration(m.ServiceWatchInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			m.ServiceWatchInterval, err)
		return
	}

	for {
		select {
		case <-m.done:
			return
		case <-time.After(watchInterval):
			m.watchServicesHeartbeat()
		}
	}
}

// Close close the master
func (w *Master) Close() {
	w.done <- struct{}{}
}
