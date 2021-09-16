package layer4rawserver

import (
	"strings"
	"sync"

	"github.com/megaease/easegress/pkg/context"
)

var (
	ProxyMap = sync.Map{}
)

func GetProxyMapKey(raddr, laddr string) string {
	var builder strings.Builder
	builder.WriteString(raddr)
	builder.WriteString(":")
	builder.WriteString(laddr)
	return builder.String()
}

func SetUDPProxyMap(key string, layer4Context context.Layer4Context) {
	ProxyMap.Store(key, layer4Context)
}

func DelUDPProxyMap(key string) {
	ProxyMap.Delete(key)
}
