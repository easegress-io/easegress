package httpserver

import (
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"golang.org/x/net/netutil"
)

const (
	checkFailedTimeout = 10 * time.Second

	stateNil     stateType = "nil"
	stateFailed            = "failed"
	stateRunning           = "running"
	stateClosed            = "closed"
)

var (
	errNil = fmt.Errorf("")
)

type (
	stateType string

	eventStart       struct{}
	eventCheckFailed struct{}
	eventServeFailed struct {
		startNum uint64
		err      error
	}
	eventReload struct{ nextSpec *Spec }
	eventClose  struct{ done chan struct{} }

	// Runtime contains all runtime info of HTTPServer.
	Runtime struct {
		handlers  *sync.Map
		spec      *Spec
		server    *http.Server
		mux       *mux
		startNum  uint64
		eventChan chan interface{}

		// status
		state atomic.Value // stateType
		err   atomic.Value // error
	}

	// Status contains all status gernerated by runtime, for displaying to users.
	Status struct {
		State stateType `yaml:"state"`
		Error string    `yaml:"error,omitempty"`
	}

	// Handler is handler handling HTTPContext.
	Handler interface {
		Handle(context.HTTPContext)
	}
)

// NewRuntime creates an HTTPServer Runtime.
func NewRuntime(handlers *sync.Map) *Runtime {
	r := &Runtime{
		handlers:  handlers,
		eventChan: make(chan interface{}, 10),
	}

	r.setState(stateNil)
	r.setError(errNil)

	go r.fsm()
	go r.checkFailed()

	return r
}

// Close closes Runtime.
func (r *Runtime) Close() {
	done := make(chan struct{})
	r.eventChan <- &eventClose{done: done}
	<-done
}

// Status returns HTTPServer Status.
func (r *Runtime) Status() *Status {
	return &Status{
		State: r.getState(),
		Error: r.getError().Error(),
	}
}

// FSM is the finite-state-machine for the Runtime.
func (r *Runtime) fsm() {
	for e := range r.eventChan {
		switch e := e.(type) {
		case *eventStart:
			r.handleEventStart(e)
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

func (r *Runtime) handleEventStart(e *eventStart) {
	r.startServer()
}

func (r *Runtime) reload(nextSpec *Spec) {
	// NOTE: Due to the mechanism of scheduler,
	// nextSpec must not be nil, just defensive programming here.
	switch {
	case r.spec == nil && nextSpec == nil:
		logger.Errorf("BUG: nextSpec is nil")
		// Nothing to do.
	case r.spec == nil && nextSpec != nil:
		r.spec = nextSpec
		r.startServer()
	case r.spec != nil && nextSpec == nil:
		logger.Errorf("BUG: nextSpec is nil")
		r.spec = nil
		r.closeServer()
	case r.spec != nil && nextSpec != nil:
		if r.needRestartServer(nextSpec) {
			r.spec = nextSpec
			r.closeServer()
			r.startServer()
		} else {
			r.spec = nextSpec
			r.mux.reloadRules(r.spec)
		}
	}
}

func (r *Runtime) setState(state stateType) {
	r.state.Store(state)
}

func (r *Runtime) getState() stateType {
	return r.state.Load().(stateType)
}

func (r *Runtime) setError(err error) {
	if err == nil {
		r.err.Store(errNil)
	} else {
		// NOTE: For type safe.
		r.err.Store(fmt.Errorf("%v", err))
	}
}

func (r *Runtime) getError() error {
	err := r.err.Load()
	if err == nil {
		return nil
	}
	return err.(error)
}

func (r *Runtime) needRestartServer(nextSpec *Spec) bool {
	x := *r.spec
	y := *nextSpec
	x.Rules, y.Rules = nil, nil

	// The update of rules need not to shutdown server.
	return !reflect.DeepEqual(x, y)
}

func (r *Runtime) startServer() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", r.spec.Port))
	if err != nil {
		r.setState(stateFailed)
		r.setError(err)

		return
	}

	limitListener := netutil.LimitListener(listener, int(r.spec.MaxConnections))

	mux := newMux(r.spec, r.handlers)
	srv := &http.Server{
		Addr:        fmt.Sprintf(":%d", r.spec.Port),
		Handler:     mux,
		IdleTimeout: time.Duration(r.spec.KeepAliveSeconds) * time.Second,
	}
	srv.SetKeepAlivesEnabled(r.spec.KeepAlive)

	if r.spec.HTTPS {
		tlsConfig, _ := r.spec.tlsConfig()
		srv.TLSConfig = tlsConfig
	}

	r.server = srv
	r.mux = mux
	r.startNum++
	r.setState(stateRunning)
	r.setError(nil)

	go func(https bool, startNum uint64) {
		var err error
		if https {
			err = r.server.ServeTLS(limitListener, "", "")
		} else {
			err = r.server.Serve(limitListener)
		}
		if err != http.ErrServerClosed {
			r.eventChan <- &eventServeFailed{
				err:      err,
				startNum: startNum,
			}
		}
	}(r.spec.HTTPS, r.startNum)
}

func (r *Runtime) closeServer() {
	if r.server == nil {
		return
	}
	// NOTE: It's safe to shutdown serve failed server.
	ctx, cancelFunc := serverShutdownContext()
	defer cancelFunc()
	err := r.server.Shutdown(ctx)
	if err != nil {
		logger.Warnf("shutdown httpserver %s failed: %v",
			r.spec.Name, err)
	}
}

func (r *Runtime) checkFailed() {
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

func (r *Runtime) handleEventCheckFailed(e *eventCheckFailed) {
	if r.getState() == stateFailed {
		r.startServer()
	}
}

func (r *Runtime) handleEventServeFailed(e *eventServeFailed) {
	if r.startNum > e.startNum {
		return
	}
	r.setState(stateFailed)
	r.setError(e.err)
}

func (r *Runtime) handleEventReload(e *eventReload) {
	r.reload(e.nextSpec)
}

func (r *Runtime) handleEventClose(e *eventClose) {
	r.closeServer()
	close(e.done)
}
