package task

import (
	"fmt"
	"reflect"
	"time"
)

type TaskStatus string

const (
	Pending             TaskStatus = "Pending"
	Running             TaskStatus = "Running"
	ResponseImmediately TaskStatus = "ResponseImmediately" // error occurred in plugin
	Finishing           TaskStatus = "Finishing"           // plugin required finish pipeline normally
	Finished            TaskStatus = "Finished"
)

// Task is designed a finite state machine:
//                   Pending
//                      +
//                      |
//                      v
//                   Running
//                      +
//                      |
//          +-----------------------+
//          v                       v
// ResponseImmediately          Finishing
//          +                       +
//          +-----------------------+
//                      |
//                      v
//                   Finished

type TaskFinished func(task Task, originalStatus TaskStatus)
type TaskRecovery func(task Task, errorPluginName string) (bool, Task)

type Task interface {
	// Finish sets status to `Finishing`
	Finish()
	// returns flag representing finished
	Finished() bool
	// ResultCode returns current result code
	ResultCode() TaskResultCode
	// Status returns current task status
	Status() TaskStatus
	// SetError sets error message, result code, and status to `ResponseImmediately`
	SetError(err error, resultCode TaskResultCode)
	// Error returns error message
	Error() error
	// StartAt returns task start time
	StartAt() time.Time
	// FinishAt return task finish time
	FinishAt() time.Time
	// AddFinishedCallback adds callback function executing after task status set to Finished
	// Callbacks are only used by plugin instead of model.
	AddFinishedCallback(name string, callback TaskFinished) TaskFinished
	// DeleteFinishedCallback deletes registered Finished callback function
	DeleteFinishedCallback(name string) TaskFinished
	// AddRecoveryFunc adds callback function executing after task status set to `ResponseImmediately`,
	// after executing them the status of task will be recovered to `Running`
	AddRecoveryFunc(name string, taskRecovery TaskRecovery) TaskRecovery
	// DeleteRecoveryFunc deletes registered recovery function
	DeleteRecoveryFunc(name string) TaskRecovery
	// Value saves task-life-cycle value, key must be comparable
	Value(key interface{}) interface{}
	// Cancel returns a cancellation channel which could be closed to broadcast cancellation of task,
	// if a plugin needs relatively long time to wait I/O or anything else,
	// it should listen this channel to exit current plugin instance.
	Cancel() <-chan struct{}
	// CancelCause returns error message of cancellation
	CancelCause() error
	// Deadline returns deadline of task if the boolean flag set true, or it's not a task with deadline cancellation
	Deadline() (time.Time, bool)
}

////

var (
	Canceled         = fmt.Errorf("context canceled")
	DeadlineExceeded = fmt.Errorf("context deadline exceeded")
)

////

func WithValue(parent Task, key, value interface{}) (Task, error) {
	if key == nil {
		return parent, fmt.Errorf("key is nil")
	}

	if !reflect.TypeOf(key).Comparable() {
		return parent, fmt.Errorf("key is not comparable")
	}

	return &ValueTask{parent, key, value}, nil
}

type ValueTask struct {
	Task
	key, value interface{}
}

func (t *ValueTask) String() string {
	return fmt.Sprintf("%v.WithValue(%#v, %#v)", t.Task, t.key, t.value)
}

func (t *ValueTask) Value(key interface{}) interface{} {
	if t.key == key {
		return t.value
	}
	return t.Task.Value(key)
}

////

func ToString(value interface{}) string {
	// limit 128 is an gateway global configuration?
	switch value.(type) {
	case []uint8:
		return fmt.Sprintf("%.128s", value)
	case string:
		return fmt.Sprintf("%.128s", value)
	default:
		return fmt.Sprintf("%.128v", value)
	}
}
