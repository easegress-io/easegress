package tcpserver

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/layer4stat"
	"github.com/megaease/easegress/pkg/util/limitlistener"
)

type runtime struct {
	superSpec *supervisor.Spec
	spec      *Spec
	startNum  uint64
	eventChan chan interface{}

	// status
	state atomic.Value // stateType
	err   atomic.Value // error

	tcpstat       *layer4stat.Layer4Stat
	limitListener *limitlistener.LimitListener
}

func (r *runtime) Close() {
	done := make(chan struct{})
	r.eventChan <- &eventClose{done: done}
	<-done
}

func newRuntime(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) *runtime {
	r := &runtime{
		superSpec: superSpec,
		eventChan: make(chan interface{}, 10),
	}

	r.setState(stateNil)
	r.setError(errNil)

	go r.fsm()
	//	go r.checkFailed()

	return r
}

func (r *runtime) setState(state stateType) {
	r.state.Store(state)
}

func (r *runtime) getState() stateType {
	return r.state.Load().(stateType)
}

func (r *runtime) setError(err error) {
	if err == nil {
		r.err.Store(errNil)
	} else {
		// NOTE: For type safe.
		r.err.Store(fmt.Errorf("%v", err))
	}
}

func (r *runtime) getError() error {
	err := r.err.Load()
	if err == nil {
		return nil
	}
	return err.(error)
}

// FSM is the finite-state-machine for the runtime.
func (r *runtime) fsm() {
	for e := range r.eventChan {
		switch e := e.(type) {
		case *eventCheckFailed:
			r.handleEventCheckFailed(e)
		case *eventServeFailed:
			r.handleEventServeFailed(e)
		case *eventReload:
			r.handleEventReload(e)
		case *eventClose:
			r.handleEventClose(e)
			// NOTE: We don't close hs.eventChan,
			// in case of panic of any other goroutines
			// to send event to it later.
			return
		default:
			logger.Errorf("BUG: unknown event: %T\n", e)
		}
	}
}

func (r *runtime) handleEventCheckFailed(e *eventCheckFailed) {

}

func (r *runtime) handleEventServeFailed(e *eventServeFailed) {
	if r.startNum > e.startNum {
		return
	}
	r.setState(stateFailed)
	r.setError(e.err)
}

func (r *runtime) handleEventReload(e *eventReload) {

}

func (r *runtime) handleEventClose(e *eventClose) {

}

func (r *runtime) checkFailed() {
	ticker := time.NewTicker(checkFailedTimeout)
	for range ticker.C {
		state := r.getState()
		if state == stateFailed {
			r.eventChan <- &eventCheckFailed{}
		} else if state == stateClosed {
			ticker.Stop()
			return
		}
	}
}
