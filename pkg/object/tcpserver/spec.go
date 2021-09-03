package tcpserver

import (
	"fmt"

	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (
	// Spec describes the TcpServer.
	Spec struct {
		Port           uint16 `yaml:"port" jsonschema:"required,minimum=1"`
		MaxConnections uint32 `yaml:"maxConnections" jsonschema:"omitempty,minimum=1"`

		// By default, backlog is set to -1 on FreeBSD, DragonFly BSD, and macOS, and to 511 on other platforms.
		Backlog             int32  `yaml:"backlog" jsonschema:"omitempty,minimum=-1"`
		SendBuf             uint32 `yaml:"sendBuf" jsonschema:"omitempty"`
		RecvBuf             uint32 `yaml:"recvBuf" jsonschema:"omitempty"`
		Reuseport           bool   `yaml:"reuseport" jsonschema:"omitempty"`
		KeepAlive           bool   `yaml:"keepAlive" jsonschema:"required"`
		TcpNodelay          bool   `yaml:"tcpNodelay" jsonschema:"omitempty"`
		ProxyConnectTimeout uint32 `yaml:"proxyConnectTimeout" jsonschema:"omitempty"`
		ProxyTimeout        uint32 `yaml:"proxyTimeout" jsonschema:"omitempty"`

		// Support multiple certs, preserve the certbase64 and keybase64
		// for backward compatibility
		SSL        bool   `yaml:"ssl" jsonschema:"omitempty"`
		CertBase64 string `yaml:"certBase64" jsonschema:"omitempty,format=base64"`
		KeyBase64  string `yaml:"keyBase64" jsonschema:"omitempty,format=base64"`

		IPFilter *ipfilter.Spec `yaml:"ipFilter,omitempty" jsonschema:"omitempty"`

		Tracing *tracing.Spec `yaml:"tracing" jsonschema:"omitempty"`
	}
)

// Validate validates TcpServerSpec.
func (spec *Spec) Validate() error {
	if spec.SSL {
		if spec.CertBase64 == "" || spec.KeyBase64 == "" {
			return fmt.Errorf("tcp proxy ssl parameters is incomplete")
		}
		// TODO need check ssl parameters
	}
	return nil
}
