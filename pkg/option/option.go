package option

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/version"

	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v2"
)

var (
	// Global needs be assigned by server/main.go,
	// referenced by any other packages if needed.
	Global *Options
)

// Options is the startup options.
type Options struct {
	yamlStr string

	ShowVersion     bool   `yaml:"-" short:"v" long:"version" description:"Print the version and exit."`
	ShowConfig      bool   `yaml:"-" short:"c" long:"print-config" description:"Print the configuration."`
	ConfigFile      string `yaml:"-" short:"f" long:"config-file" description:"Load server configuration from a file(yaml format), other command line flags will be ignored if specified."`
	ForceNewCluster bool   `yaml:"-" long:"force-new-cluster" description:"Force to create a new one-member cluster."`

	// If a config file is specified, below command line flags will be ignored.

	// meta
	Name                  string `yaml:"name" long:"name" description:"Human-readable name for this member."`
	ClusterName           string `yaml:"cluster-name" long:"cluster-name" description:"Human-readable name for the new cluster, ignored while joining an existed cluster."`
	ClusterRole           string `yaml:"cluster-role" long:"cluster-role" description:"Cluster role for this member. (reader, writer)"`
	ClusterRequestTimeout string `yaml:"cluster-request-timeout" long:"cluster-request-timeout" description:"Timeout to handle request to the cluster."`
	ClusterClientURL      string `yaml:"cluster-client-url" long:"cluter-client-url" description:"URL to listen on for cluster client traffic."`
	ClusterPeerURL        string `yaml:"cluster-peer-url" long:"cluter-peer-url" description:"URL to listen on for cluster peer traffic."`
	ClusterJoinURLs       string `yaml:"cluster-join-urls" long:"cluster-join-urls" description:"One or more URLs of the writers in cluster to join, delimited by ',' without whitespaces"`
	APIAddr               string `yaml:"api-addr" long:"api-addr" description:"Address([host]:port) to listen on for administration traffic."`

	HomeDir   string `yaml:"home-dir" long:"home-dir" description:"Path to the home directory."`
	DataDir   string `yaml:"data-dir" long:"data-dir" description:"Path to the data directory."`
	WALDir    string `yaml:"wal-dir" long:"wal-dir" description:"Path to the WAL directory."`
	LogDir    string `yaml:"log-dir" long:"log-dir" description:"Path to the log directory."`
	MemberDir string `yaml:"member-dir" long:"member-dir" description:"Path to the member store directory."`

	// Prepare them in advance.
	AbsHomeDir   string `yaml:"-"`
	AbsDataDir   string `yaml:"-"`
	AbsWALDir    string `yaml:"-"`
	AbsLogDir    string `yaml:"-"`
	AbsMemberDir string `yaml:"-"`

	Debug bool `yaml:"debug" long:"debug" description:"Flag to set lowest log level from INFO downgrade DEBUG."`

	// profile
	CPUProfileFile    string `yaml:"cpu-profile-file" long:"cpu-profile-file" description:"Path to the CPU profile file."`
	MemoryProfileFile string `yaml:"memory-profile-file" long:"memory-profile-file" description:"Path to the memory profile file."`

	// pidfile
	PidFile string `yaml:"pid-file" long:"pid-file" description:"Path to file for systemctl PidFile."`

	// go test may fail with out '-t', reference: https://github.com/alecthomas/kingpin/issues/167
	Placeholder string `yaml:"-" short:"t" long:"test" description:"not used yet"`
}

// New creates a default Options.
func New() *Options {
	return &Options{
		Name:                  "eg-name",
		ClusterName:           "eg-cluster-name",
		ClusterRole:           "writer",
		ClusterRequestTimeout: "10s",
		ClusterClientURL:      "http://localhost:2379",
		ClusterPeerURL:        "http://localhost:2380",
		APIAddr:               "localhost:2381",

		HomeDir:   "./",
		DataDir:   "data",
		LogDir:    "log",
		MemberDir: "member",
		Debug:     false,
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

	err = opt.prepare()
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
		if opt.ForceNewCluster {
			return fmt.Errorf("reader got force-new-cluster")
		}

		if opt.ClusterJoinURLs == "" {
			return fmt.Errorf("reader got empty cluster-join-urls")
		}

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

	_, err := time.ParseDuration(opt.ClusterRequestTimeout)
	if err != nil {
		return fmt.Errorf("invalid cluster-request-timeout: %v", err)
	}

	_, _, err = net.SplitHostPort(opt.APIAddr)
	if err != nil {
		return fmt.Errorf("invalid api-addr: %v", err)
	}

	if err != nil {
		return fmt.Errorf("invalid api-url: %v", err)
	}

	// dirs
	if opt.HomeDir == "" {
		return fmt.Errorf("homw-dir")
	}
	if opt.DataDir == "" {
		return fmt.Errorf("empty data-dir")
	}
	if opt.LogDir == "" {
		return fmt.Errorf("empty log-dir")
	}
	if opt.MemberDir == "" {
		return fmt.Errorf("empty member-dir")
	}

	// profile: nothing to validate

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

func (opt *Options) prepare() error {
	abs, isAbs, clean, join := filepath.Abs, filepath.IsAbs, filepath.Clean, filepath.Join
	if isAbs(opt.HomeDir) {
		opt.AbsHomeDir = clean(opt.HomeDir)
	} else {
		absHomeDir, err := abs(opt.HomeDir)
		if err != nil {
			return err
		}
		opt.AbsHomeDir = absHomeDir
	}

	type dirItem struct {
		dir    string
		absDir *string
	}
	table := []dirItem{
		{dir: opt.DataDir, absDir: &opt.AbsDataDir},
		{dir: opt.WALDir, absDir: &opt.AbsWALDir},
		{dir: opt.LogDir, absDir: &opt.AbsLogDir},
		{dir: opt.MemberDir, absDir: &opt.AbsMemberDir},
	}
	for _, di := range table {
		if di.dir == "" {
			continue
		}
		if filepath.IsAbs(di.dir) {
			*di.absDir = clean(di.dir)
		} else {
			*di.absDir = clean(join(opt.AbsHomeDir, di.dir))
		}
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
