package cluster

import (
	"testing"
)

func testRequestEvent(t *testing.T, srcNode, dstNode string, f *Future, r *RequestEvent) {
	if r.requestId != f.requestId {
		t.Fatalf("src node %s, dst node %s, expected requestId: %d, but got: %d", srcNode, dstNode, f.requestId, r.requestId)
	}
	if r.requestTime != f.requestTime {
		t.Fatalf("src node %s, dst node %s, expected requestTime: %v, but got: %v", srcNode, dstNode, f.requestTime, r.requestTime)
	}
}
