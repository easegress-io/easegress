package backend

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"

	httpstat "github.com/tcnksm/go-httpstat"
)

type (
	request struct {
		std        *http.Request
		statResult *httpstat.Result
		endTime    *time.Time
	}

	resultState struct {
		buff *bytes.Buffer
	}
)

func (p *pool) newRequest(ctx context.HTTPContext, server *Server, reqBody io.Reader) (*request, error) {
	req := &request{
		statResult: &httpstat.Result{},
	}

	r := ctx.Request()

	url := server.URL + r.Path()
	if r.Query() != "" {
		url += "?" + r.Query()
	}
	stdr, err := http.NewRequest(r.Method(), url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("BUG: new request failed: %v", err)
	}
	stdr.Header = r.Header().Std()

	stdr = stdr.WithContext(httpstat.WithHTTPStat(stdr.Context(), req.statResult))

	req.std = stdr

	return req, nil
}

func (r *request) finish() {
	if r.endTime != nil {
		logger.Errorf("finished alreyad")
		return
	}

	now := time.Now()
	r.statResult.End(now)
	r.endTime = &now
}

func (r *request) total() time.Duration {
	if r.endTime == nil {
		logger.Errorf("call total before finish")
		return r.statResult.Total(time.Now())
	}

	return r.statResult.Total(*r.endTime)
}

func (r *request) detail() string {
	rs := &resultState{buff: bytes.NewBuffer(nil)}
	r.statResult.Format(rs, 's')
	return rs.buff.String()
}

func (rs *resultState) Write(p []byte) (int, error) { return rs.buff.Write(p) }
func (rs *resultState) Width() (int, bool)          { return 0, false }
func (rs *resultState) Precision() (int, bool)      { return 0, false }
func (rs *resultState) Flag(c int) bool             { return false }
