package statussynccontroller

import (
	"runtime/debug"
	"sync"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/storage"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/util/timetool"

	"gopkg.in/yaml.v2"
)

const (
	// Category is the category of StatusSyncController.
	Category = supervisor.CategorySystemController

	// Kind is the kind of StatusSyncController.
	Kind = "StatusSyncController"

	maxStatusesRecordCount = 10
)

type (
	// StatusSyncController is a system controller to synchronize
	// status of every object to remote storage.
	StatusSyncController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		storage *storage.Storage
		timer   *timetool.DistributedTimer
		// sorted by timestamp in ascending order
		statusesRecords      []*StatusesRecord
		StatusesRecordsMutex sync.RWMutex

		done chan struct{}
	}

	// Spec describes StatusSyncController.
	Spec struct {
	}

	// StatusesRecord is the history record for status of every running object.
	StatusesRecord struct {
		Statuses     map[string]*supervisor.Status
		UnixTimestmp int64
	}
)

func marshalStatus(status *supervisor.Status) ([]byte, error) {
	buff, err := yaml.Marshal(status.ObjectStatus)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	err = yaml.Unmarshal(buff, &m)
	if err != nil {
		return nil, err
	}

	m["timestamp"] = status.Timestamp

	buff, err = yaml.Marshal(m)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func init() {
	supervisor.Register(&StatusSyncController{})
}

// Category returns the category of StatusSyncController.
func (ssc *StatusSyncController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of StatusSyncController.
func (ssc *StatusSyncController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of StatusSyncController.
func (ssc *StatusSyncController) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes StatusSyncController.
func (ssc *StatusSyncController) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	ssc.superSpec, ssc.spec, ssc.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	ssc.reload()
}

// Inherit inherits previous generation of StatusSyncController.
func (ssc *StatusSyncController) Inherit(spec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	previousGeneration.Close()
	ssc.Init(spec, super)
}

func (ssc *StatusSyncController) reload() {
	ssc.timer = timetool.NewDistributedTimer(nextSyncStatusDuration)
	ssc.storage = storage.New(ssc.super.Options(), ssc.super.Cluster())
	ssc.done = make(chan struct{})

	go ssc.run()
}

func (ssc *StatusSyncController) run() {
	for {
		select {
		case t := <-ssc.timer.C:
			ssc.syncStatus(t.Unix())
		case <-ssc.done:
			return
		}
	}
}

// Status returns the status of StatusSyncController.
func (ssc *StatusSyncController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: struct{}{},
	}
}

// Close closes StatusSyncController.
func (ssc *StatusSyncController) Close() {
	close(ssc.done)
	ssc.storage.Close()
	ssc.timer.Close()
}

func (ssc *StatusSyncController) syncStatus(unixTimestamp int64) {
	statuses := make(map[string]string)
	statusesRecord := &StatusesRecord{
		Statuses:     make(map[string]*supervisor.Status),
		UnixTimestmp: unixTimestamp,
	}

	walkFn := func(runningObject *supervisor.RunningObject) bool {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("recover from syncStatus, err: %v, stack trace:\n%s\n",
					err, debug.Stack())
			}
		}()

		name := runningObject.Spec().Name()

		status := runningObject.Instance().Status()
		status.Timestamp = unixTimestamp

		statusesRecord.Statuses[name] = status

		buff, err := marshalStatus(status)
		if err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v",
				status, err)
			return false
		}
		statuses[name] = string(buff)

		return true
	}

	ssc.super.WalkRunningObjects(walkFn, supervisor.CategoryAll)

	ssc.addStatusesRecord(statusesRecord)
	ssc.storage.SyncStatus(statuses)
}

func (ssc *StatusSyncController) addStatusesRecord(statusesRecord *StatusesRecord) {
	ssc.StatusesRecordsMutex.Lock()
	defer ssc.StatusesRecordsMutex.Unlock()

	ssc.statusesRecords = append(ssc.statusesRecords, statusesRecord)
	if len(ssc.statusesRecords) > maxStatusesRecordCount {
		ssc.statusesRecords = ssc.statusesRecords[1:]
	}
}

// GetStatusesRecords return the latest statuses records.
func (ssc *StatusSyncController) GetStatusesRecords() []*StatusesRecord {
	ssc.StatusesRecordsMutex.RLock()
	defer ssc.StatusesRecordsMutex.RUnlock()

	records := make([]*StatusesRecord, len(ssc.statusesRecords))
	for i, record := range ssc.statusesRecords {
		records[i] = record
	}

	return records
}
