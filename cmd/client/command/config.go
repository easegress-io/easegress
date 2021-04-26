package command

type EaseGatewayConfig struct {
	Name              string `yaml:"name" jsonschema:"required"`
	ClusterName       string `yaml:"cluster-name" jsonschema:"required"`
	ClusterRole       string `yaml:"cluster-role" jsonschema:"required"`
	ClusterClientUrl  string `yaml:"cluster-client-url" jsonschema:"required"`
	ClusterPeerUrl    string `yaml:"cluster-peer-url" jsonschema:"required"`
	ClusterJoinUrls   string `yaml:"cluster-join-urls" jsonschema:"required"`
	ApiAddr           string `yaml:"api-addr" jsonschema:"required"`
	DataDir           string `yaml:"data-dir" jsonschema:"required"`
	WalDir            string `yaml:"wal-dir" wal-dir:"required"`
	CpuProfileFile    string `yaml:"cpu-profile-file" jsonschema:"required"`
	MemoryProfileFile string `yaml:"memory-profile-file" jsonschema:"required"`
	LogDir            string `yaml:"log-dir" jsonschema:"required"`
	MemberDir         string `yaml:"member-dir" jsonschema:"required"`
	StdLogLevel       string `yaml:"std-log-level" jsonschema:"required"`
}

type MeshControllerConfig struct {
	Name              string `yaml:"name" jsonschema:"required"`
	Kind              string `yaml:"kind" jsonschema:"required"`
	RegistryType      string `yaml:"registryType" jsonschema:"required"`
	HeartbeatInterval string `yaml:"heartbeatInterval" jsonschema:"required"`
}

type MeshOperatorConfig struct {
	ClusterName          string `yaml:"cluster-name" jsonschema:"required"`
	ClusterJoinURLs      string `yaml:"cluster-join-urls" jsonschema:"required"`
	MetricsAddr          string `yaml:"metrics-bind-address" jsonschema:"required"`
	EnableLeaderElection bool   `yaml:"leader-elect" jsonschema:"required"`
	ProbeAddr            string `yaml:"health-probe-bind-address" jsonschema:"required"`
}

type EaseGatewayReaderParams struct {
	ClusterJoinUrls       string            `yaml:"cluster-join-urls" jsonschema:"required"`
	ClusterRequestTimeout string            `yaml:"cluster-request-timeout" jsonschema:"required"`
	ClusterRole           string            `yaml:"cluster-role" jsonschema:"required"`
	ClusterName           string            `yaml:"cluster-name" jsonschema:"required"`
	Name                  string            `yaml:"name" jsonschema:"required"`
	Labels                map[string]string `yaml:"Labels" jsonschema:"required"`
}
