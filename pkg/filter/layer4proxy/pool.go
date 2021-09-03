package layer4proxy

import (
	"fmt"
	"github.com/google/martian/log"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/layer4stat"
	"github.com/megaease/easegress/pkg/util/memorycache"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"io"
	"net"
	"time"
)

type (
	protocol string

	pool struct {
		spec *PoolSpec

		tagPrefix     string
		writeResponse bool

		servers     *servers
		layer4stat  *layer4stat.Layer4Stat
		memoryCache *memorycache.MemoryCache
	}

	// PoolSpec describes a pool of servers.
	PoolSpec struct {
		Protocol        protocol          `yaml:"protocol" jsonschema:"required" `
		SpanName        string            `yaml:"spanName" jsonschema:"omitempty"`
		ServersTags     []string          `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
		Servers         []*Server         `yaml:"servers" jsonschema:"omitempty"`
		ServiceRegistry string            `yaml:"serviceRegistry" jsonschema:"omitempty"`
		ServiceName     string            `yaml:"serviceName" jsonschema:"omitempty"`
		LoadBalance     *LoadBalance      `yaml:"loadBalance" jsonschema:"required"`
		MemoryCache     *memorycache.Spec `yaml:"memoryCache,omitempty" jsonschema:"omitempty"`
	}

	// PoolStatus is the status of Pool.
	PoolStatus struct {
		Stat *layer4stat.Status `yaml:"stat"`
	}
)

// Validate validates poolSpec.
func (s PoolSpec) Validate() error {
	if s.ServiceName == "" && len(s.Servers) == 0 {
		return fmt.Errorf("both serviceName and servers are empty")
	}

	serversGotWeight := 0
	for _, server := range s.Servers {
		if server.Weight > 0 {
			serversGotWeight++
		}
	}
	if serversGotWeight > 0 && serversGotWeight < len(s.Servers) {
		return fmt.Errorf("not all servers have weight(%d/%d)",
			serversGotWeight, len(s.Servers))
	}

	if s.ServiceName == "" {
		servers := newStaticServers(s.Servers, s.ServersTags, s.LoadBalance)
		if servers.len() == 0 {
			return fmt.Errorf("serversTags picks none of servers")
		}
	}

	return nil
}

func newPool(super *supervisor.Supervisor, spec *PoolSpec, tagPrefix string, writeResponse bool) *pool {

	var memoryCache *memorycache.MemoryCache
	if spec.MemoryCache != nil {
		memoryCache = memorycache.New(spec.MemoryCache)
	}

	return &pool{
		spec: spec,

		tagPrefix:     tagPrefix,
		writeResponse: writeResponse,

		servers:     newServers(super, spec),
		layer4stat:  layer4stat.New(),
		memoryCache: memoryCache,
	}
}

func (p *pool) status() *PoolStatus {
	s := &PoolStatus{Stat: p.layer4stat.Status()}
	return s
}

func (p *pool) handle(ctx context.Layer4Context, clientConn *net.TCPConn) string {
	addTag := func(subPrefix, msg string) {
		tag := stringtool.Cat(p.tagPrefix, "#", subPrefix, ": ", msg)
		ctx.Lock()
		ctx.AddTag(tag)
		ctx.Unlock()
	}

	server, err := p.servers.next(ctx)
	if err != nil {
		addTag("serverErr", err.Error())
		return resultInternalError
	}
	addTag("addr", server.HostPort)

	rawConn, err := net.DialTimeout("tcp", server.HostPort, 1000*time.Millisecond)
	if err != nil {
		log.Errorf("dial tcp for addr: % failed, err: %v", server.HostPort, err)
	}
	backendConn := rawConn.(*net.TCPConn)

	defer func(backendConn *net.TCPConn) {
		closeErr := backendConn.Close()
		if closeErr != nil {
			logger.Warnf("close backend conn for %v failed, err: %v", server.HostPort, err)
		}
	}(backendConn)

	errChan := make(chan error)
	go p.connCopy(backendConn, clientConn, errChan)
	go p.connCopy(clientConn, backendConn, errChan)

	err = <-errChan
	if err != nil {
		logger.Errorf("Error during connection: %v", err)
	}

	err = <-errChan // TODO export tcp config for backend conn, watch client/backend error

	ctx.Lock()
	defer ctx.Unlock()
	// NOTE: The code below can't use addTag and setStatusCode in case of deadlock.

	return ""
}

func (p *pool) close() {
	p.servers.close()
}

func (p *pool) connCopy(dst *net.TCPConn, src *net.TCPConn, errCh chan error) {
	_, err := io.Copy(dst, src)
	errCh <- err

	errClose := dst.CloseWrite()
	if errClose != nil {
		logger.Debugf("Error while terminating connection: %v", errClose)
	}
}
