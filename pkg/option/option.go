package option

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"strings"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/version"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"

	flags "github.com/jessevdk/go-flags"
)

var (
	Global     *Options
	GlobalYAML string
	GlobalJSON string
)

func init() {
	// Set default value in one place.
	Global = &Options{
		Name:             "member-001",
		ClusterName:      "cluster-A",
		ClusterRole:      "writer",
		ClusterClientURL: "http://localhost:2379",
		ClusterPeerURL:   "http://localhost:2380",
		APIAddr:          "localhost:2381",

		DataDir: "./data",
		LogDir:  "./logs",

		StdLogLevel: "INFO",

		PipelineInitParallelism: 1,
		PipelineMinParallelism:  5,
		PipelineMaxParallelism:  5120,

		PluginIODataFormatLengthLimit: 128,
		PluginPythonRootNamespace:     false,
		PluginShellRootNamespace:      false,
		CGIDir:                        "./cgi",
		CertDir:                       "./cert",
	}

	_, err := flags.NewParser(Global, flags.HelpFlag|flags.PassDoubleDash).Parse()
	if err != nil {
		if err, ok := err.(*flags.Error); ok && err.Type == flags.ErrHelp {
			common.Exit(0, err.Message)
		}
		common.Exit(1, err.Error())
	}

	if Global.ShowVersion {
		common.Exit(0, version.Short)
	}

	if Global.ConfigFile != "" {
		buff, err := ioutil.ReadFile(Global.ConfigFile)
		if err != nil {
			common.Exit(1, fmt.Sprintf("read config file failed: %v", err))
		}
		err = yaml.Unmarshal(buff, Global)
		if err != nil {
			common.Exit(1, fmt.Sprintf("unmarshal config file %s to yaml failed: %v",
				Global.ConfigFile, err))
		}
	}

	err = Global.validate()
	if err != nil {
		common.Exit(1, err.Error())
	}

	buff, err := json.Marshal(Global)
	if err != nil {
		common.Exit(1, fmt.Sprintf("marshal config to json failed: %v", err))
	}

	buff, err = yaml.Marshal(Global)
	if err != nil {
		common.Exit(1, fmt.Sprintf("marshal config to yaml failed: %v", err))
	}
	if Global.ShowConfig {
		fmt.Printf("%s", buff)
	}

	GlobalYAML = string(buff)
}

type Options struct {
	ShowVersion bool   `json:"-" yaml:"-" short:"v" long:"version" description:"Print the version and exit."`
	ShowConfig  bool   `json:"-" yaml:"-" short:"c" long:"print-config" description:"Print the configuration."`
	ConfigFile  string `json:"-" yaml:"-" short:"f" long:"config-file" description:"Load server configuration from a file(yaml format), other command line flags will be ignored if specified."`

	// If a config file is specified, below command line flags will be ignored.

	// meta
	Name             string `json:"name" yaml:"name" long:"name" description:"Human-readable name for this member."`
	ClusterName      string `json:"cluter-name" yaml:"cluster-name" long:"cluster-name" description:"Human-readable name for the new cluster, ignored while joining an existed cluster."`
	ClusterRole      string `json:"cluter-role" yaml:"cluster-role" long:"cluster-role" description:"Cluster role for this member. (reader, writer)"`
	ForceNewCluster  bool   `json:"force-new-cluster" yaml:"force-new-cluster" long:"force-new-cluster" description:"It starts a new cluster even if previously started; unsafe."`
	ClusterClientURL string `json:"cluter-client-url" yaml:"cluster-client-url" long:"cluter-client-url" description:"URL to listen on for cluster client traffic."`
	ClusterPeerURL   string `json:"cluter-peer-url" yaml:"cluster-peer-url" long:"cluter-peer-url" description:"URL to listen on for cluster peer traffic."`
	ClusterJoinURL   string `json:"cluster-join-url" yaml:"cluster-join-url" long:"cluster-join-url" description:"URL of one member in the existed cluster to join."`
	APIAddr          string `json:"api-addr" yaml:"api-addr" long:"api-addr" description:"Address([host]:port) to listen on for administration traffic."`

	// store
	DataDir string `json:"data-dir" yaml:"data-dir" long:"data-dir" description:"Path to the data directory."`
	WALDir  string `json:"wal-dir" yaml:"wal-dir" long:"wal-dir" description:"Path to the WAL directory."`

	// log
	LogDir      string `json:"log-dir" yaml:"log-dir" long:"log-dir" description:"Path to the log directory."`
	StdLogLevel string `json:"std-log-level" yaml:"std-log-level" long:"std-log-level" description:"Set standard log level."`

	// profile
	CPUProfileFile    string `json:"cpu-profile-file" yaml:"cpu-profile-file" long:"cpu-profile-file" description:"Path to the CPU profile file."`
	MemoryProfileFile string `json:"memory-profile-file" yaml:"memory-profile-file" long:"memory-profile-file" description:"Path to the memory profile file."`

	// pipeline
	PipelineInitParallelism uint32 `json:"pipeline-init-parallelism" yaml:"pipeline-init-parallelism" long:"pipeline-init-parallelism" description:"Initial parallelism for a pipeline running in dynamic schedule mode."`
	PipelineMinParallelism  uint32 `json:"pipeline-min-parallelism" yaml:"pipeline-min-parallelism" long:"pipeline-min-parallelism" description:"Minimum parallelism for a pipeline running in dynamic schedule mode."`
	PipelineMaxParallelism  uint32 `json:"pipeline-max-parallelism" yaml:"pipeline-max-parallelism" long:"pipeline-max-parallelism" description:"Maximum parallelism for a pipeline running in dynamic schedule mode."`

	// plugin
	PluginIODataFormatLengthLimit uint64 `json:"plugin-io-data-format-len-limit" yaml:"plugin-io-data-format-len-limit" long:"plugin-io-data-format-len-limit" description:"Bytes limitation on plugin IO data formation output."`
	PluginPythonRootNamespace     bool   `json:"plugin-python-root-namespace" yaml:"plugin-python-root-namespace" long:"plugin-python-root-namespace" description:"Run python code in root namespace without isolation."`
	PluginShellRootNamespace      bool   `json:"plugin-shell-root-namespace" yaml:"plugin-shell-root-namespace" long:"plugin-shell-root-namespace" description:"Run shell code in root namespace without isolation."`
	CGIDir                        string `json:"cgi-dir" yaml:"cgi-dir" long:"cgi-dir" description:"Path to the CGI directory."`
	CertDir                       string `json:"cert-dir" yaml:"cert-dir" long:"cert-dir" description:"Path to the Certificate directory."`
}

func (o *Options) validate() error {
	// meta
	if o.Name == "" {
		return fmt.Errorf("empty name")
	}
	if err := common.ValidateName(o.Name); err != nil {
		return err
	}

	if o.ClusterName == "" {
		if o.ClusterJoinURL == "" {
			return fmt.Errorf("empty cluster-name for a new cluster")
		}
	} else if err := common.ValidateName(o.ClusterName); err != nil {
		return err
	}

	switch o.ClusterRole {
	case "reader":
		_, err := url.Parse(o.ClusterJoinURL)
		if err != nil {
			return fmt.Errorf("invalid cluster-join-url: %v", err)
		}
	case "writer":
		_, err := url.Parse(o.ClusterPeerURL)
		if err != nil {
			return fmt.Errorf("invalid cluster-peer-url: %v", err)
		}
		_, err = url.Parse(o.ClusterClientURL)
		if err != nil {
			return fmt.Errorf("invalid cluster-client-url: %v", err)
		}

		if o.ClusterJoinURL != "" {
			if o.ForceNewCluster {
				return fmt.Errorf("force new cluster is conflict with join an existed cluster")
			}
			_, err = url.Parse(o.ClusterJoinURL)
			if err != nil {
				return fmt.Errorf("invalid cluster-join-url: %v", err)
			}
		}
	default:
		return fmt.Errorf("invalid cluster-role(support writer, reader)")
	}

	_, _, err := net.SplitHostPort(o.APIAddr)
	if err != nil {
		return fmt.Errorf("invalid api-addr: %v", err)
	}

	if err != nil {
		return fmt.Errorf("invalid api-url: %v", err)
	}

	// store
	if o.DataDir == "" {
		return fmt.Errorf("empty data-dir")
	}
	if o.ClusterRole == "writer" {
		var dirs []string
		if !common.IsDirEmpty(o.DataDir) {
			dirs = append(dirs, o.DataDir)
		}
		if o.WALDir != "" && !common.IsDirEmpty(o.WALDir) {
			dirs = append(dirs, o.WALDir)
		}
		if len(dirs) != 0 {
			if o.ClusterJoinURL == "" {
				if !o.ForceNewCluster {
					return fmt.Errorf("%s is not empty, "+
						"use flag force-new-cluster to clean historical member info(data is inherited), "+
						"or backup/clean the directory",
						strings.Join(dirs, ","))
				}
			} else {
				return fmt.Errorf("%s is not empty, will be conflict with joining an existed cluster"+
					"please backup/clean the directory",
					strings.Join(dirs, ","))
			}
		}
	}

	// log
	if _, err := logrus.ParseLevel(o.StdLogLevel); err != nil {
		return fmt.Errorf("invalid std-log-level: %v", err)
	}
	if o.LogDir == "" {
		return fmt.Errorf("empty log-dir")
	}

	// profile
	// nothing to validate

	// pipeline
	if o.PipelineInitParallelism < 1 ||
		o.PipelineInitParallelism > uint32(^uint16(0)) {
		return fmt.Errorf("invalid pipeline-init-parallelism[1,%d]", ^uint16(0))
	}

	if o.PipelineMaxParallelism < 1 ||
		o.PipelineMaxParallelism > 102400 {
		return fmt.Errorf("incalid pipeline-max-parallelism[1,102400]")
	}

	if o.PipelineMinParallelism > o.PipelineMaxParallelism {
		return fmt.Errorf("pipeline-min-parallelism %d > pipeline-max-parallelism %d",
			o.PipelineMinParallelism, o.PipelineMaxParallelism)
	}

	// plugin
	if o.PluginIODataFormatLengthLimit < 1 ||
		o.PluginIODataFormatLengthLimit > 10000009 {
		return fmt.Errorf("invalid plugin-io-data-format-len-limit[1,10000009]")
	}

	return nil
}
