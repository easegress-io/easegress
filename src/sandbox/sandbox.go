package sandbox

import (
	"logger"
)

// NOTICE: Currently sandbox just shows a demo idea whose main goal is to
// prevent gateway from crashing at running code of third-party plugin.

// Usage Example:
// s := sandbox.New(fmt.Sprintf("contruct plugin %s", p.conf.PluginName()))
// defer s.DeferFunc()
// instance, err := p.constructor(p.conf)
// s.Close()

// An instance of Sandbox is disposable.
type Sandbox struct {
	area   string
	closed bool
}

func New(area string) *Sandbox {
	return &Sandbox{
		area:   area,
		closed: false,
	}
}

func (s *Sandbox) DeferFunc() {
	if s.closed {
		return
	}

	if r := recover(); r != nil {
		logger.Errorf("[sandbox prevent %s from panic: %v]", s.area, r)
	}
}

func (s *Sandbox) Close() {
	s.closed = true
}
