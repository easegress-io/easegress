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
	Finish()
	Finished() bool
	ResultCode() TaskResultCode
	Status() TaskStatus
	SetError(err error, resultCode TaskResultCode)
	Error() error
	StartAt() time.Time
	FinishAt() time.Time

	// Callbacks are only used by plugin instead of model
	AddFinishedCallback(name string, callback TaskFinished) TaskFinished
	DeleteFinishedCallback(name string) TaskFinished
	AddRecoveryFunc(name string, taskRecovery TaskRecovery) TaskRecovery
	DeleteRecoveryFunc(name string) TaskRecovery

	Value(key interface{}) interface{}
	Cancel() <-chan struct{}
	CancelCause() error
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
