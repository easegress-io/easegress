/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package statussynccontroller

import (
	"runtime/debug"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/timetool"

	"github.com/megaease/easegress/pkg/object/rawconfigtrafficcontroller"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
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
		superSpec *supervisor.Spec
		spec      *Spec

		timer            *timetool.DistributedTimer
		lastSyncStatuses map[string]string
		// sorted by timestamp in ascending order
		statusesRecords      []*StatusesRecord
		StatusesRecordsMutex sync.RWMutex

		// statusUpdateMaxBatchSize is maximum statuses to update in one cluster transaction
		statusUpdateMaxBatchSize int

		done chan struct{}
	}

	// Spec describes StatusSyncController.
	Spec struct{}

	// StatusesRecord is the history record for status of every running object.
	StatusesRecord struct {
		Statuses      map[string]*supervisor.Status
		UnixTimestamp int64
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

	if m == nil {
		m = map[string]interface{}{}
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
func (ssc *StatusSyncController) Init(superSpec *supervisor.Spec) {
	ssc.superSpec, ssc.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	ssc.reload()
}

// Inherit inherits previous generation of StatusSyncController.
func (ssc *StatusSyncController) Inherit(spec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	ssc.Init(spec)
}

func (ssc *StatusSyncController) reload() {
	ssc.timer = timetool.NewDistributedTimer(nextSyncStatusDuration)
	ssc.done = make(chan struct{})

	opts := ssc.superSpec.Super().Options()
	ssc.statusUpdateMaxBatchSize = opts.StatusUpdateMaxBatchSize
	if ssc.statusUpdateMaxBatchSize < 1 {
		ssc.statusUpdateMaxBatchSize = 20
	}
	logger.Infof("StatusUpdateMaxBatchSize is %d", ssc.statusUpdateMaxBatchSize)

	go ssc.run()
}

func (ssc *StatusSyncController) run() {
	for {
		select {
		case t := <-ssc.timer.C:
			ssc.handleStatus(t.Unix())
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
	ssc.timer.Close()
}

func safeMarshal(value *supervisor.Status) (string, bool) {
	buff, err := marshalStatus(value)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v",
			value, err)
		return "", false
	}
	return string(buff), true
}

// key: namespace/kind/objectName
func (ssc *StatusSyncController) splitNamespaceStatus(status *trafficcontroller.StatusInSameNamespace,
	targetStatuses map[string]string, targetStatusesRecord *StatusesRecord) bool {

	for key, value := range status.ToSyncStatus() {
		name := ssc.superSpec.Super().Cluster().Layout().FullObjectName(status.Namespace, key)
		targetStatusesRecord.Statuses[name] = value

		marshalledValue, ok := safeMarshal(value)
		if !ok {
			return false
		}
		targetStatuses[name] = marshalledValue
	}
	return true
}

func (ssc *StatusSyncController) handleStatus(unixTimestamp int64) {
	statuses := make(map[string]string)
	statusesRecord := &StatusesRecord{
		Statuses:      make(map[string]*supervisor.Status),
		UnixTimestamp: unixTimestamp,
	}

	walkFn := func(entity *supervisor.ObjectEntity) bool {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("recover from syncStatus, err: %v, stack trace:\n%s\n",
					err, debug.Stack())
			}
		}()

		name := entity.Spec().Name()

		status := entity.Instance().Status()
		status.Timestamp = unixTimestamp

		if trafficStatus, ok := status.ObjectStatus.(*trafficcontroller.Status); ok {
			statusInNamespaces := trafficStatus.Specs
			for _, statInNS := range statusInNamespaces {
				if !ssc.splitNamespaceStatus(statInNS, statuses, statusesRecord) {
					return false
				}
			}
			return true
		} else if rawTrafficStatus, ok := status.ObjectStatus.(*rawconfigtrafficcontroller.Status); ok {
			return ssc.splitNamespaceStatus(rawTrafficStatus, statuses, statusesRecord)
		} else {
			statusesRecord.Statuses[name] = status
			marshalledValue, ok := safeMarshal(status)
			if !ok {
				return false
			}
			statuses[name] = marshalledValue
		}
		return true
	}

	ssc.superSpec.Super().WalkControllers(walkFn)

	ssc.addStatusesRecord(statusesRecord)
	ssc.syncStatusToCluster(statuses)
}

func (ssc *StatusSyncController) syncStatusToCluster(statuses map[string]string) {
	// Delete statuses which disappeared in current status.
	if ssc.lastSyncStatuses != nil {
		kvs := make(map[string]*string)
		for k := range ssc.lastSyncStatuses {
			if _, exists := statuses[k]; !exists {
				k = ssc.superSpec.Super().Cluster().Layout().StatusObjectKey(k)
				kvs[k] = nil
			}
		}
		err := ssc.superSpec.Super().Cluster().PutAndDeleteUnderLease(kvs)
		if err != nil {
			logger.Errorf("sync status failed. If the message size is too large, "+
				"please increase the value of cluster.MaxCallSendMsgSize in configuration: %v", err)
		}
	}

	ssc.lastSyncStatuses = statuses

	kvs := make(map[string]*string)
	for k, value := range statuses {
		key := ssc.superSpec.Super().Cluster().Layout().StatusObjectKey(k)
		v := value
		kvs[key] = &v
		if len(kvs) >= ssc.statusUpdateMaxBatchSize {
			err := ssc.superSpec.Super().Cluster().PutAndDeleteUnderLease(kvs)
			if err != nil {
				logger.Errorf("sync status failed. If the message size is too large, "+
					"please increase the value of cluster.MaxCallSendMsgSize in configuration: %v", err)
			}
			kvs = make(map[string]*string)
		}
	}
	if len(kvs) > 0 {
		err := ssc.superSpec.Super().Cluster().PutAndDeleteUnderLease(kvs)
		if err != nil {
			logger.Errorf("sync status failed. If the message size is too large, "+
				"please increase the value of cluster.MaxCallSendMsgSize in configuration: %v", err)
		}
	}
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
	copy(records, ssc.statusesRecords)
	return records
}
