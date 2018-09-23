package gateway

import (
	"fmt"
	"time"

	"github.com/hexdecteam/easegateway/pkg/cluster"
)

func newRequestParam(nodeNames []string, group string, mode Mode, timeout time.Duration) *cluster.RequestParam {
	requestParam := cluster.RequestParam{
		TargetNodeNames: nodeNames,
		TargetNodeTags:  map[string]string{},
		Timeout:         timeout,
		// FIXME(shengdong) parameterize ResponseRelayCount when needed
		ResponseRelayCount: 1, // fault tolerance on network issue
	}

	if group != "" {
		requestParam.TargetNodeTags[groupTagKey] = fmt.Sprintf("^%s$", group)
	}
	if mode != NilMode {
		requestParam.TargetNodeTags[modeTagKey] = fmt.Sprintf("^%s$", mode.String())
	}
	return &requestParam
}

// pagination takes the size(length) of iteams, and the page and limit on request,
// returns the subscripts [start, end) which indicate the sub-items under the rule of pagination.
// The final return value [start, end) is safe even an error occurs.
func pagination(length int, page, limit uint32) (start, end int, err error) {
	if page == 0 {
		return 0, 0, fmt.Errorf("BUG: invalid zero value for page")
	}
	if limit == 0 {
		return 0, 0, fmt.Errorf("BUG: invalid zero value for limit")
	}

	start = int(int64(page-1) * int64(limit))
	end = start + int(limit)

	if start > length-1 {
		return 0, 0, fmt.Errorf("the scope of index [%d, %d) totally goes beyond [0, %d)",
			start, end, length)
	} else if end > length {
		err = fmt.Errorf("the scope of index [%d, %d) partially goes beyond [0, %d)",
			start, end, length)
		end = length
		return start, end, err
	}

	return
}
