package model

import (
	"fmt"
	"sync"
	"time"

	"common"
	"task"
)

////

type Task struct {
	// FIXME: Add locking for task parallel handling when needed
	// sync.RWMutex
	data                    []byte
	startAt                 *time.Time
	finishAt                *time.Time
	resultCode              task.TaskResultCode
	status                  task.TaskStatus
	err                     error
	statusFinishedCallbacks []*common.NamedCallback
	taskRecoveries          []*common.NamedCallback
}

func NewTask() *Task {
	return &Task{
		status:     task.Pending,
		resultCode: task.ResultOK,
	}
}

func (t *Task) Finish() {
	t.setStatus(task.Finishing)
}

func (t *Task) Finished() bool {
	return t.finishAt != nil
}

func (t *Task) ResultCode() task.TaskResultCode {
	return t.resultCode
}

func (t *Task) Status() task.TaskStatus {
	return t.status
}

func (t *Task) SetError(err error, resultCode task.TaskResultCode) {
	if t.err != nil {
		return // never do again
	}

	if err == nil {
		err = fmt.Errorf("unknown error")
	}

	if !task.ValidResultCode(resultCode) || task.SuccessfulResult(resultCode) {
		resultCode = task.ResultUnknownError
	}

	t.err = err
	// Set result code first, there are callbacks in setStatus()
	t.resultCode = resultCode
	t.setStatus(task.ResponseImmediately)
}

func (t *Task) Error() error {
	return t.err
}

func (t *Task) StartAt() time.Time {
	if t.startAt != nil {
		return *t.startAt
	} else {
		return time.Time{}
	}
}
func (t *Task) FinishAt() time.Time {
	if t.finishAt != nil {
		return *t.finishAt
	} else {
		return time.Time{}
	}
}

func (t *Task) AddFinishedCallback(name string, callback task.TaskFinished) task.TaskFinished {
	var oriCallback interface{}
	t.statusFinishedCallbacks, oriCallback, _ = common.AddCallback(t.statusFinishedCallbacks, name, callback, true)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(task.TaskFinished)
	}
}

func (t *Task) DeleteFinishedCallback(name string) task.TaskFinished {
	var oriCallback interface{}
	t.statusFinishedCallbacks, oriCallback = common.DeleteCallback(t.statusFinishedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(task.TaskFinished)
	}
}

func (t *Task) AddRecoveryFunc(name string, taskRecovery task.TaskRecovery) task.TaskRecovery {
	var oriCallback interface{}
	t.taskRecoveries, oriCallback, _ = common.AddCallback(t.taskRecoveries, name, taskRecovery, true)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(task.TaskRecovery)
	}
}

func (t *Task) DeleteRecoveryFunc(name string) task.TaskRecovery {
	var oriCallback interface{}
	t.taskRecoveries, oriCallback = common.DeleteCallback(t.taskRecoveries, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(task.TaskRecovery)
	}
}

func (t *Task) Value(key interface{}) interface{} {
	return nil
}

func (t *Task) Cancel() <-chan struct{} {
	return nil
}

func (t *Task) CancelCause() error {
	return nil
}

func (t *Task) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (t *Task) start() error {
	if t.startAt != nil {
		return fmt.Errorf("task is already started at %s", t.startAt.String())
	}

	now := time.Now()
	t.startAt = &now
	t.setStatus(task.Running)
	return nil
}

func (t *Task) finish(latestTask task.Task) error {
	if t.startAt == nil {
		return fmt.Errorf("task was not started")
	} else if t.finishAt != nil {
		return fmt.Errorf("task is already finished at %s", t.finishAt.String())
	}

	now := time.Now()
	t.finishAt = &now
	t.setStatus(task.Finished)

	// clean up, remove references to cooperate with go gc
	t.taskRecoveries = t.taskRecoveries[:0]

	oriStatus := t.status

	tmp := make([]*common.NamedCallback, len(t.statusFinishedCallbacks))
	copy(tmp, t.statusFinishedCallbacks)

	for _, namedCallback := range tmp {
		namedCallback.Callback().(task.TaskFinished)(latestTask, oriStatus)
	}

	return nil
}

func (t *Task) setStatus(status task.TaskStatus) {
	t.status = status
}

func (t *Task) clearError(originalCode task.TaskResultCode) {
	t.err = nil
	t.resultCode = originalCode
	t.setStatus(task.Running)
}

func (t *Task) recover(errorPluginName string, lastStatus task.TaskStatus, latestTask task.Task) (bool, task.Task) {
	tmp := make([]*common.NamedCallback, len(t.taskRecoveries))
	copy(tmp, t.taskRecoveries)

	for _, namedCallback := range tmp {
		recovered, t1 := namedCallback.Callback().(task.TaskRecovery)(latestTask, errorPluginName)
		if recovered {
			if lastStatus == task.Running { // defensive
				t.clearError(task.ResultOK)
			}
			t.setStatus(lastStatus)
			return true, t1
		}
	}

	return false, latestTask
}

////

type cancelFunc func()

type canceler interface {
	cancel(removeFromParent bool, err error)
	Cancel() <-chan struct{}
}

func parentCancelTask(parent task.Task) (*cancelTask, bool) {
	for {
		switch c := parent.(type) {
		case *cancelTask:
			return c, true
		case *timerTask:
			return c.cancelTask, true
		case *task.ValueTask:
			parent = c.Task
		default:
			return nil, false
		}
	}
}

func propagateCancel(parent task.Task, child canceler) {
	if parent.Cancel() == nil {
		return // parent is never canceled
	}

	if p, ok := parentCancelTask(parent); ok {
		p.Lock()
		if p.err != nil { // parent has already been canceled
			child.cancel(false, p.err)
		} else {
			p.children = append(p.children, child)
		}
		p.Unlock()
	} else {
		go func() {
			select {
			case <-parent.Cancel():
				child.cancel(false, parent.CancelCause())
			case <-child.Cancel():
			}
		}()
	}
}

func removeChild(parent task.Task, child canceler) {
	p, ok := parentCancelTask(parent)
	if !ok {
		return
	}

	p.Lock()
	if p.children != nil {
		for i, c := range p.children {
			if c == child {
				p.children = append(p.children[:i], p.children[i+1:]...)
				break
			}
		}
	}
	p.Unlock()
}

////

func withCancel(parent task.Task) (task.Task, cancelFunc) {
	c := newCancelTask(parent)
	propagateCancel(parent, c)
	return c, func() { c.cancel(true, task.Canceled) }
}

type cancelTask struct {
	task.Task
	sync.Mutex
	done     chan struct{}
	err      error
	children []canceler
}

func newCancelTask(parent task.Task) *cancelTask {
	return &cancelTask{
		Task: parent,
		done: make(chan struct{}),
	}
}

func (c *cancelTask) cancel(removeFromParent bool, err error) {
	if err == nil {
		fmt.Errorf("missing cancel error")
	}

	c.Lock()
	if c.err != nil {
		c.Unlock()
		return // already canceled
	}
	c.err = err
	close(c.done)
	for _, child := range c.children {
		child.cancel(false, err)
	}
	c.children = nil
	c.Unlock()

	if removeFromParent {
		removeChild(c.Task, c)
	}
}

func (c *cancelTask) Cancel() <-chan struct{} {
	return c.done
}

func (c *cancelTask) CancelCause() error {
	c.Lock()
	defer c.Unlock()
	return c.err
}

////

func withDeadline(parent task.Task, deadline time.Time) (task.Task, cancelFunc) {
	if cur, ok := parent.Deadline(); ok && cur.Before(deadline) {
		// The current deadline is already sooner than the new one.
		return withCancel(parent)
	}

	c := &timerTask{
		cancelTask: newCancelTask(parent),
		deadline:   deadline,
	}

	propagateCancel(parent, c)

	d := time.Until(deadline)
	if d <= 0 { // deadline has already passed
		c.cancel(true, task.DeadlineExceeded)
		return c, func() { c.cancel(true, task.Canceled) }
	}

	c.Lock()
	defer c.Unlock()

	if c.err == nil {
		c.timer = time.AfterFunc(d, func() { c.cancel(true, task.DeadlineExceeded) })
	}

	return c, func() { c.cancel(true, task.Canceled) }
}

type timerTask struct {
	*cancelTask
	timer    *time.Timer // Under cancelTask.lock
	deadline time.Time
}

func (c *timerTask) Deadline() (time.Time, bool) {
	return c.deadline, true
}

func (c *timerTask) cancel(removeFromParent bool, err error) {
	c.cancelTask.cancel(false, err)

	if removeFromParent {
		removeChild(c.cancelTask.Task, c)
	}

	c.Lock()
	defer c.Unlock()

	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}

////

func withTimeout(parent task.Task, timeout time.Duration) (task.Task, cancelFunc) {
	return withDeadline(parent, time.Now().Add(timeout))
}
