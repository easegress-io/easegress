package httpserver

import (
	stdcontext "context"
	"time"
)

const (
	serverShutdownTimeout = 30 * time.Second
)

func serverShutdownContext() (stdcontext.Context, stdcontext.CancelFunc) {
	ctx, cancelFunc := stdcontext.WithTimeout(stdcontext.Background(), serverShutdownTimeout)
	return ctx, cancelFunc
}
