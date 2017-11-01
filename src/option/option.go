package option

import (
	"flag"
	"os"
	"strings"
	"time"

	"common"
)

var (
	// cluster stuff
	ClusterHost           string
	ClusterGroup          string
	MemberMode            string
	MemberName            string
	Peers                 []string
	OPLogMaxSeqGapToPull  uint16
	OPLogPullMaxCountOnce uint16
	OPLogPullInterval     time.Duration
	OPLogPullTimeout      time.Duration

	RestHost                       string
	Stage                          string
	ConfigHome, LogHome            string
	CpuProfileFile, MemProfileFile string
	ShowVersion                    bool

	PluginIODataFormatLengthLimit uint64
	PluginPythonRootNamespace     bool
	PluginShellRootNamespace      bool
)

func init() {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "node0"
	}

	clusterHost := flag.String("cluster_host", "localhost", "specify cluster listen host")
	clusterGroup := flag.String("group", "default", "specify cluster group name")
	memberMode := flag.String("mode", "read", "specify member mode (read or write)")
	memberName := flag.String("name", hostName, "specify member name")
	peers := flag.String("peers", "", "specify address list of peer members (separated by comma)")
	opLogMaxSeqGapToPull := new(uint16)
	flag.Var(common.NewUint16Value(5, opLogMaxSeqGapToPull), "oplog_max_seq_gap_to_pull",
		"specify max gap of sequence of operation logs deciding whether to wait for missing operations or not")
	opLogPullMaxCountOnce := new(uint16)
	flag.Var(common.NewUint16Value(5, opLogPullMaxCountOnce), "oplog_pull_max_count_once",
		"specify max count of pulling operation logs once")
	opLogPullInterval := new(uint16)
	flag.Var(common.NewUint16Value(10, opLogPullInterval), "oplog_pull_interval",
		"specify interval of pulling operation logs in second")
	opLogPullTimeout := new(uint16)
	flag.Var(common.NewUint16Value(30, opLogPullTimeout), "oplog_pull_timeout",
		"specify timeout of pulling operation logs in second")

	restHost := flag.String("rest_host", "localhost", "specify rest listen host")
	stage := flag.String("stage", "debug", "sepcify runtime stage (debug, test, prod)")
	configHome := flag.String("config", common.CONFIG_HOME_DIR, "specify config home path")
	logHome := flag.String("log", common.LOG_HOME_DIR, "specify log home path")
	cpuProfileFile := flag.String("cpuprofile", "", "specify cpu profile output file, "+
		"cpu profiling will be fully disabled if not provided")
	memProfileFile := flag.String("memprofile", "", "specify heap dump file, "+
		"memory profiling will be fully disabled if not provided")
	showVersion := flag.Bool("version", false, "output version information")

	pluginIODataFormatLengthLimit := new(uint64)
	flag.Var(common.NewUint64RangeValue(128, pluginIODataFormatLengthLimit, 1, 10000009), // limited by fmt precision
		"plugin_io_data_format_len_limit", "specify length limit on plugin IO data formation output in byte unit")
	pluginPythonRootNamespace := flag.Bool("plugin_python_root_namespace", false,
		"specify if to run python code in root namespace without isolation")
	pluginShellRootNamespace := flag.Bool("plugin_shell_root_namespace", false,
		"specify if to run shell script in root namespace without isolation")

	flag.Parse()

	ClusterHost = *clusterHost
	ClusterGroup = *clusterGroup
	MemberMode = *memberMode
	MemberName = *memberName
	OPLogMaxSeqGapToPull = *opLogMaxSeqGapToPull
	OPLogPullMaxCountOnce = *opLogPullMaxCountOnce
	OPLogPullInterval = time.Duration(*opLogPullInterval) * time.Second
	OPLogPullTimeout = time.Duration(*opLogPullTimeout) * time.Second
	Peers = make([]string, 0)
	for _, peer := range strings.Split(*peers, ",") {
		peer = strings.TrimSpace(peer)
		if len(peer) > 0 {
			Peers = append(Peers, peer)
		}
	}

	RestHost = *restHost
	Stage = *stage
	ConfigHome = *configHome
	LogHome = *logHome
	CpuProfileFile = *cpuProfileFile
	MemProfileFile = *memProfileFile
	ShowVersion = *showVersion

	PluginIODataFormatLengthLimit = *pluginIODataFormatLengthLimit
	PluginPythonRootNamespace = *pluginPythonRootNamespace
	PluginShellRootNamespace = *pluginShellRootNamespace
}
