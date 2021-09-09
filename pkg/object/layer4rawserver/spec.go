package layer4rawserver

import (
	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (
	// Spec describes the TcpServer.
	Spec struct {
		Protocol       string `yaml:"protocol" jsonschema:"required,enum=tcp,enum=udp"`
		LocalAddr      string `yaml:"localAddr" jsonschema:"omitempty"`
		Port           uint16 `yaml:"port" json:"port" jsonschema:"required"`
		BindPort       bool   `yaml:"bindPort" jsonschema:"omitempty"`
		MaxConnections uint32 `yaml:"maxConnections" jsonschema:"omitempty,minimum=1"`

		// By default, backlog is set to -1 on FreeBSD, DragonFly BSD, and macOS, and to 511 on other platforms.
		Backlog             int32  `yaml:"backlog" jsonschema:"omitempty,minimum=-1"`
		SendBuf             int    `yaml:"sendBuf" jsonschema:"omitempty"`
		RecvBuf             int    `yaml:"recvBuf" jsonschema:"omitempty"`
		Reuseport           bool   `yaml:"reuseport" jsonschema:"omitempty"`
		KeepAlive           bool   `yaml:"keepAlive" jsonschema:"required"`
		TcpNodelay          bool   `yaml:"tcpNodelay" jsonschema:"omitempty"`
		ProxyConnectTimeout uint32 `yaml:"proxyConnectTimeout" jsonschema:"omitempty"`
		ProxyTimeout        uint32 `yaml:"proxyTimeout" jsonschema:"omitempty"`

		IPFilter *ipfilter.Spec `yaml:"ipFilter,omitempty" jsonschema:"omitempty"`
	}

	ListenerConfig struct {
		Protocol  string `yaml:"protocol" jsonschema:"required,enum=tcp,enum=udp"`
		LocalAddr string `yaml:"localAddr" jsonschema:"omitempty"`
		Port      uint16 `yaml:"port" json:"port" jsonschema:"required"`
		BindPort  bool   `yaml:"bindPort" jsonschema:"omitempty"`
		// By default, backlog is set to -1 on FreeBSD, DragonFly BSD, and macOS, and to 511 on other platforms.
		Backlog             int32  `yaml:"backlog" jsonschema:"omitempty,minimum=-1"`
		SendBuf             int    `yaml:"sendBuf" jsonschema:"omitempty"`
		RecvBuf             int    `yaml:"recvBuf" jsonschema:"omitempty"`
		Reuseport           bool   `yaml:"reuseport" jsonschema:"omitempty"`
		KeepAlive           bool   `yaml:"keepAlive" jsonschema:"required"`
		TcpNodelay          bool   `yaml:"tcpNodelay" jsonschema:"omitempty"`
		ProxyConnectTimeout uint32 `yaml:"proxyConnectTimeout" jsonschema:"omitempty"`
		ProxyTimeout        uint32 `yaml:"proxyTimeout" jsonschema:"omitempty"`
	}
)

// Validate validates TcpServerSpec.
func (spec *Spec) Validate() error {
	return nil
}
