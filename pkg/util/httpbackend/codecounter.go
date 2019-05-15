package httpbackend

import (
	"sync"
	"sync/atomic"

	"github.com/megaease/easegateway/pkg/logger"
)

type codeCounter struct {
	//	   server  code:count
	counter map[string]*sync.Map
}

func newCodeCounter(servers []Server) *codeCounter {
	counter := make(map[string]*sync.Map)
	for _, server := range servers {
		counter[server.URL] = &sync.Map{}
	}
	return &codeCounter{
		counter: counter,
	}
}

func (cc *codeCounter) count(server *Server, code int) {
	c, exists := cc.counter[server.URL]
	if !exists {
		logger.Errorf("BUG: count server %s not found", server)
		return
	}

	var count uint64
	counter, _ := c.LoadOrStore(code, &count)
	atomic.AddUint64(counter.(*uint64), 1)
}

func (cc *codeCounter) codes() map[string]map[int]uint64 {
	codes := make(map[string]map[int]uint64)
	for server, c := range cc.counter {
		codes[server] = make(map[int]uint64)
		c.Range(func(k, v interface{}) bool {
			codes[server][k.(int)] = atomic.LoadUint64(v.(*uint64))

			return true
		})
	}

	return codes
}
