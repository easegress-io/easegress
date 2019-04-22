package option

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/version"

	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v2"
)

// Options is the startup options.
type Options struct {
	yamlStr string

	ShowVersion       bool   `yaml:"-" short:"v" long:"version" description:"Print the version and exit."`
	ShowConfig        bool   `yaml:"-" short:"c" long:"print-config" description:"Print the configuration."`
	ConfigFile        string `yaml:"-" short:"f" long:"config-file" description:"Load server configuration from a file(yaml format), other command line flags will be ignored if specified."`
	IsBootstrapWriter bool   `yaml:"-" short:"b" long:"is-bootstrap-writer" description:"Instruct the node to create the genesis embedded etcd server. This flag can only be set on ONE node during the installation. It will take no effect after the initialization finished and will be ignored."`

	// If a config file is specified, below command line flags will be ignored.

	// meta
	Name             string `yaml:"name" long:"name" description:"Human-readable name for this member."`
	ClusterName      string `yaml:"cluster-name" long:"cluster-name" description:"Human-readable name for the new cluster, ignored while joining an existed cluster."`
	ClusterRole      string `yaml:"cluster-role" long:"cluster-role" description:"Cluster role for this member. (reader, writer)"`
	ForceNewCluster  bool   `yaml:"force-new-cluster" long:"force-new-cluster" description:"It starts a new cluster even if previously started; unsafe."`
	ClusterClientURL string `yaml:"cluster-client-url" long:"cluter-client-url" description:"URL to listen on for cluster client traffic."`
	ClusterPeerURL   string `yaml:"cluster-peer-url" long:"cluter-peer-url" description:"URL to listen on for cluster peer traffic."`
	ClusterJoinURLs  string `yaml:"cluster-join-urls" long:"cluster-join-urls" description:"One or more URLs of the writers in cluster to join, delimited by ',' without whitespaces"`
	APIAddr          string `yaml:"api-addr" long:"api-addr" description:"Address([host]:port) to listen on for administration traffic."`

	// store
	DataDir string `yaml:"data-dir" long:"data-dir" description:"Path to the data directory."`
	WALDir  string `yaml:"wal-dir" long:"wal-dir" description:"Path to the WAL directory."`

	// log
	LogDir string `yaml:"log-dir" long:"log-dir" description:"Path to the log directory."`

	//conf
	ConfDir string `yaml:"conf-dir" long:"conf-dir" description:"Path to the configuration directory."`

	Debug bool `yaml:"debug" long:"debug" description:"Flag to set lowest log level from INFO downgrade DEBUG."`

	// profile
	CPUProfileFile    string `yaml:"cpu-profile-file" long:"cpu-profile-file" description:"Path to the CPU profile file."`
	MemoryProfileFile string `yaml:"memory-profile-file" long:"memory-profile-file" description:"Path to the memory profile file."`

	// etcd
	EtcdRequestTimeoutInMilli int64 `yaml:"etcd-request-timeout-in-milli" long:"etcd-request-timeout-in-milli" description:"Timeout in milli seconds to access the etcd server."`

	// go test may fail with out '-t', reference: https://github.com/alecthomas/kingpin/issues/167
	Placeholder string `yaml:"-" short:"t" long:"test" description:"not used yet"`
}

// New creates a default Options.
func New() *Options {
	return &Options{
		Name:              "eg-name",
		ClusterName:       "eg-cluster-name",
		ClusterRole:       "writer",
		ClusterClientURL:  "http://localhost:2379",
		ClusterPeerURL:    "http://localhost:2380",
		APIAddr:           "localhost:2381",
		IsBootstrapWriter: false,

		DataDir: "./data",

		LogDir:  "./logs",
		ConfDir: "./conf",
		Debug:   false,

		EtcdRequestTimeoutInMilli: 1000,
	}
}

// YAML returns yaml string of option, need to be called after calling Parse.
func (opt *Options) YAML() string {
	return opt.yamlStr
}

// Parse parses all arguments, returns normal message without error if --help/--version set.
func (opt *Options) Parse() (string, error) {
	_, err := flags.NewParser(opt, flags.HelpFlag|flags.PassDoubleDash).Parse()
	if err != nil {
		if err, ok := err.(*flags.Error); ok && err.Type == flags.ErrHelp {
			return err.Message, nil
		}
		return "", err
	}

	if opt.ShowVersion {
		return version.Short, nil
	}

	if opt.ConfigFile != "" {
		buff, err := ioutil.ReadFile(opt.ConfigFile)
		if err != nil {
			return "", fmt.Errorf("read config file failed: %v", err)
		}
		err = yaml.Unmarshal(buff, opt)
		if err != nil {
			return "", fmt.Errorf("unmarshal config file %s to yaml failed: %v",
				opt.ConfigFile, err)
		}
	}

	err = opt.validate()
	if err != nil {
		return "", err
	}

	buff, err := yaml.Marshal(opt)
	if err != nil {
		return "", fmt.Errorf("marshal config to yaml failed: %v", err)
	}
	opt.yamlStr = string(buff)

	if opt.ShowConfig {
		fmt.Printf("%s", opt.yamlStr)
	}

	return "", nil
}

func (opt *Options) validate() error {
	if opt.ClusterName == "" {
		return fmt.Errorf("empty cluster-name")
	} else if err := common.ValidateName(opt.ClusterName); err != nil {
		return err
	}

	switch opt.ClusterRole {
	case "reader":
		for _, urlText := range strings.Split(opt.ClusterJoinURLs, ",") {
			_, err := url.Parse(urlText)
			if err != nil {
				return fmt.Errorf("invalid cluster-join-urls: %v", err)
			}
		}

	case "writer":
		_, err := url.Parse(opt.ClusterPeerURL)
		if err != nil {
			return fmt.Errorf("invalid cluster-peer-url: %v", err)
		}
		_, err = url.Parse(opt.ClusterClientURL)
		if err != nil {
			return fmt.Errorf("invalid cluster-client-url: %v", err)
		}

		if opt.ClusterJoinURLs != "" {
			if opt.ForceNewCluster {
				return fmt.Errorf("force new cluster is conflict with join an existed cluster")
			}

			for _, urlText := range strings.Split(opt.ClusterJoinURLs, ",") {
				_, err := url.Parse(urlText)
				if err != nil {
					return fmt.Errorf("invalid cluster-join-urls: %v", err)
				}
			}
			_, err = url.Parse(opt.ClusterJoinURLs)
		}
	default:
		return fmt.Errorf("invalid cluster-role(support writer, reader)")
	}

	_, _, err := net.SplitHostPort(opt.APIAddr)
	if err != nil {
		return fmt.Errorf("invalid api-addr: %v", err)
	}

	if err != nil {
		return fmt.Errorf("invalid api-url: %v", err)
	}

	// store
	if opt.DataDir == "" {
		return fmt.Errorf("empty data-dir")
	}
	if opt.ClusterRole == "writer" {
		var dirs []string
		if !common.IsDirEmpty(opt.DataDir) {
			dirs = append(dirs, opt.DataDir)
		}
		if opt.WALDir != "" && !common.IsDirEmpty(opt.WALDir) {
			dirs = append(dirs, opt.WALDir)
		}
	}

	// log
	if opt.LogDir == "" {
		return fmt.Errorf("empty log-dir")
	}

	// profile
	// nothing to validate

	// meta
	if opt.Name == "" {
		name, err := generateMemberName(opt.APIAddr)
		if err != nil {
			return err
		}
		opt.Name = name
	}
	if err := common.ValidateName(opt.Name); err != nil {
		return err
	}

	return nil
}

func generateMemberName(apiAddr string) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}

	memberName := hostname + "-" + apiAddr
	memberName = strings.Replace(memberName, ",", "-", -1)
	memberName = strings.Replace(memberName, ":", "-", -1)
	memberName = strings.Replace(memberName, "=", "-", -1)
	return memberName, nil

}
