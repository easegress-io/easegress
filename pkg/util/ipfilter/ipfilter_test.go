package ipfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIPFilterChain(t *testing.T) {
	assert := assert.New(t)

	assert.Nil(NewIPFilterChain(nil, nil))

	filters := NewIPFilterChain(nil, &Spec{
		AllowIPs: []string{"192.168.1.0/24"},
	})
	assert.NotNil(filters)

	assert.NotNil(NewIPFilterChain(filters, nil))
}

func TestNewIPFilter(t *testing.T) {
	assert := assert.New(t)
	assert.Nil(New(nil))
	assert.NotNil(New(&Spec{
		AllowIPs: []string{"192.168.1.0/24"},
	}))
}

func TestAllowIP(t *testing.T) {
	assert := assert.New(t)
	var filter *IPFilter
	assert.True(filter.Allow("192.168.1.1"))
	filter = New(&Spec{
		AllowIPs: []string{"192.168.1.0/24"},
		BlockIPs: []string{"192.168.2.0/24"},
	})
	assert.True(filter.Allow("192.168.1.1"))
	assert.False(filter.Allow("192.168.2.1"))
}
