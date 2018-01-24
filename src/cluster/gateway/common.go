package gateway

import (
	"fmt"
	"time"

	"cluster"
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
