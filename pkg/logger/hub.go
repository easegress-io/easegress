package logger

import (
	"sync"

	"go.uber.org/zap"
)

var lh *logHub

func init() {
	lh = &logHub{
		loggers: make(map[string]*zap.SugaredLogger),
		mu:      &sync.RWMutex{},
	}
}

type logHub struct {
	loggers map[string]*zap.SugaredLogger

	mu *sync.RWMutex
}

// register registers a logger with name.
func (lh *logHub) register(name string, logger *zap.SugaredLogger) {
	lh.mu.Lock()
	defer lh.mu.Unlock()

	lh.loggers[name] = logger
}

func (lh *logHub) unregister(name string) {
	lh.mu.Lock()
	defer lh.mu.Unlock()

	delete(lh.loggers, name)
}

// sync syncs all loggers.
func (lh *logHub) sync() {
	lh.mu.RLock()
	defer lh.mu.RUnlock()

	for _, logger := range lh.loggers {
		logger.Sync()
	}
}
