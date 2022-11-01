package routers

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestRuleMatch(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com:8080", nil)
	req, _ := httpprot.NewRequest(stdr)

	rule := &Rule{}
	rule.init(nil)

	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{Host: "www.megaease.com"}
	rule.init(nil)
	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{HostRegexp: `^[^.]+\.megaease\.com$`}
	rule.init(nil)
	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{HostRegexp: `^[^.]+\.megaease\.cn$`}
	rule.init(nil)
	assert.NotNil(rule)
	assert.False(rule.Match(req))
}

