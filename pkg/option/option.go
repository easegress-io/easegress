/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package option

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/common"
	"github.com/megaease/easegress/pkg/version"
)

// ClusterOptions defines the cluster members.
type ClusterOptions struct {
	// Primary members define following URLs to form a cluster.
	ListenPeerURLs           []string          `yaml:"listen-peer-urls"`
	ListenClientURLs         []string          `yaml:"listen-client-urls"`
	AdvertiseClientURLs      []string          `yaml:"advertise-client-urls"`
	InitialAdvertisePeerURLs []string          `yaml:"initial-advertise-peer-urls"`
	InitialCluster           map[string]string `yaml:"initial-cluster"`
	StateFlag                string            `yaml:"state-flag"`
	// Secondary members define URLs to connect to cluster formed by primary members.
	PrimaryListenPeerURLs []string `yaml:"primary-listen-peer-urls"`
	MaxCallSendMsgSize    int      `yaml:"max-call-send-msg-size"`
}

// Options is the start-up options.
type Options struct {
	flags   *pflag.FlagSet
	viper   *viper.Viper
	yamlStr string

	// Flags from command line only.
	ShowVersion     bool   `yaml:"-"`
	ShowHelp        bool   `yaml:"-"`
	ShowConfig      bool   `yaml:"-"`
	ConfigFile      string `yaml:"-"`
	ForceNewCluster bool   `yaml:"-"`
	SignalUpgrade   bool   `yaml:"-"`

	// If a config file is specified, below command line flags will be ignored.

	// meta
	Name                     string            `yaml:"name" env:"EG_NAME"`
	Labels                   map[string]string `yaml:"labels" env:"EG_LABELS"`
	APIAddr                  string            `yaml:"api-addr"`
	Debug                    bool              `yaml:"debug"`
	DisableAccessLog         bool              `yaml:"disable-access-log"`
	InitialObjectConfigFiles []string          `yaml:"initial-object-config-files"`

	// cluster options
	ClusterName           string         `yaml:"cluster-name"`
	ClusterRole           string         `yaml:"cluster-role"`
	ClusterRequestTimeout string         `yaml:"cluster-request-timeout"`
	Cluster               ClusterOptions `yaml:"cluster"`
	// Deprecated. Use ClusterOptions instead.
	ClusterListenClientURLs         []string `yaml:"cluster-listen-client-urls"`
	ClusterListenPeerURLs           []string `yaml:"cluster-listen-peer-urls"`
	ClusterAdvertiseClientURLs      []string `yaml:"cluster-advertise-client-urls"`
	ClusterInitialAdvertisePeerURLs []string `yaml:"cluster-initial-advertise-peer-urls"`
	ClusterJoinURLs                 []string `yaml:"cluster-join-urls"`

	// Path.
	HomeDir   string `yaml:"home-dir"`
	DataDir   string `yaml:"data-dir"`
	WALDir    string `yaml:"wal-dir"`
	LogDir    string `yaml:"log-dir"`
	MemberDir string `yaml:"member-dir"`

	// Profile.
	CPUProfileFile    string `yaml:"cpu-profile-file"`
	MemoryProfileFile string `yaml:"memory-profile-file"`

	// Status
	StatusUpdateMaxBatchSize int `yaml:"status-update-max-batch-size"`

	// Prepare the items below in advance.
	AbsHomeDir   string `yaml:"-"`
	AbsDataDir   string `yaml:"-"`
	AbsWALDir    string `yaml:"-"`
	AbsLogDir    string `yaml:"-"`
	AbsMemberDir string `yaml:"-"`
}

// addClusterVars introduces cluster arguments.
func addClusterVars(opt *Options) {
	opt.flags.StringVar(&opt.ClusterName, "cluster-name", "eg-cluster-default-name", "Human-readable name for the new cluster, ignored while joining an existed cluster.")
	opt.flags.StringVar(&opt.ClusterRole, "cluster-role", "primary", "Cluster role for this member (primary, secondary).")
	opt.flags.StringVar(&opt.ClusterRequestTimeout, "cluster-request-timeout", "10s", "Timeout to handle request in the cluster.")

	// Deprecated: Use 'Cluster connection configuration' instead.
	opt.flags.StringSliceVar(&opt.ClusterListenClientURLs, "cluster-listen-client-urls", []string{"http://localhost:2379"}, "Deprecated. Use cluster.listen-client-urls instead.")
	opt.flags.StringSliceVar(&opt.ClusterListenPeerURLs, "cluster-listen-peer-urls", []string{"http://localhost:2380"}, "Deprecated. Use cluster.listen-peer-urls instead.")
	opt.flags.StringSliceVar(&opt.ClusterAdvertiseClientURLs, "cluster-advertise-client-urls", []string{"http://localhost:2379"}, "Deprecated. Use cluster.advertise-client-urls instead.")
	opt.flags.StringSliceVar(&opt.ClusterInitialAdvertisePeerURLs, "cluster-initial-advertise-peer-urls", []string{"http://localhost:2380"}, "Deprecated. Use cluster.initial-advertise-peer-urls instead.")
	opt.flags.StringSliceVar(&opt.ClusterJoinURLs, "cluster-join-urls", nil, "Deprecated. Use cluster.initial-cluster instead.")

	// Cluster connection configuration
	opt.flags.StringSliceVar(&opt.Cluster.ListenClientURLs, "listen-client-urls", []string{"http://localhost:2379"}, "List of URLs to listen on for cluster client traffic.")
	opt.flags.StringSliceVar(&opt.Cluster.ListenPeerURLs, "listen-peer-urls", []string{"http://localhost:2380"}, "List of URLs to listen on for cluster peer traffic.")
	opt.flags.StringSliceVar(&opt.Cluster.AdvertiseClientURLs, "advertise-client-urls", []string{"http://localhost:2379"}, "List of this member's client URLs to advertise to the rest of the cluster.")
	opt.flags.StringSliceVar(&opt.Cluster.InitialAdvertisePeerURLs, "initial-advertise-peer-urls", []string{"http://localhost:2380"}, "List of this member's peer URLs to advertise to the rest of the cluster.")
	opt.flags.StringToStringVarP(&opt.Cluster.InitialCluster, "initial-cluster", "", nil,
		"List of (member name, URL) pairs that will form the cluster. E.g. primary-1=http://localhost:2380.")
	opt.flags.StringVar(&opt.Cluster.StateFlag, "state-flag", "new", "Cluster state (new, existing)")
	opt.flags.StringSliceVar(&opt.Cluster.PrimaryListenPeerURLs,
		"primary-listen-peer-urls",
		[]string{"http://localhost:2380"},
		"List of peer URLs of primary members. Define this only, when cluster-role is secondary.")
	opt.flags.IntVar(&opt.Cluster.MaxCallSendMsgSize, "max-call-send-msg-size", 10*1024*1024, "Maximum size in bytes for cluster synchronization messages.")
}

// New creates a default Options.
func New() *Options {
	opt := &Options{
		flags: pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError),
		viper: viper.New(),
	}

	opt.flags.BoolVarP(&opt.ShowVersion, "version", "v", false, "Print the version and exit.")
	opt.flags.BoolVarP(&opt.ShowHelp, "help", "h", false, "Print the helper message and exit.")
	opt.flags.BoolVarP(&opt.ShowConfig, "print-config", "c", false, "Print the configuration.")
	opt.flags.StringVarP(&opt.ConfigFile, "config-file", "f", "", "Load server configuration from a file(yaml format), other command line flags will be ignored if specified.")
	opt.flags.BoolVar(&opt.ForceNewCluster, "force-new-cluster", false, "Force to create a new one-member cluster.")
	opt.flags.BoolVar(&opt.SignalUpgrade, "signal-upgrade", false, "Send an upgrade signal to the server based on the local pid file, then exit. The original server will start a graceful upgrade after signal received.")
	opt.flags.StringVar(&opt.Name, "name", "eg-default-name", "Human-readable name for this member.")
	opt.flags.StringToStringVar(&opt.Labels, "labels", nil, "The labels for the instance of Easegress.")
	addClusterVars(opt)
	opt.flags.StringVar(&opt.APIAddr, "api-addr", "localhost:2381", "Address([host]:port) to listen on for administration traffic.")
	opt.flags.BoolVar(&opt.Debug, "debug", false, "Flag to set lowest log level from INFO downgrade DEBUG.")
	opt.flags.StringSliceVar(&opt.InitialObjectConfigFiles, "initial-object-config-files", nil, "List of configuration files for initial objects, these objects will be created at startup if not already exist.")

	opt.flags.StringVar(&opt.HomeDir, "home-dir", "./", "Path to the home directory.")
	opt.flags.StringVar(&opt.DataDir, "data-dir", "data", "Path to the data directory.")
	opt.flags.StringVar(&opt.WALDir, "wal-dir", "", "Path to the WAL directory.")
	opt.flags.StringVar(&opt.LogDir, "log-dir", "log", "Path to the log directory.")
	opt.flags.StringVar(&opt.MemberDir, "member-dir", "member", "Path to the member directory.")

	opt.flags.StringVar(&opt.CPUProfileFile, "cpu-profile-file", "", "Path to the CPU profile file.")
	opt.flags.StringVar(&opt.MemoryProfileFile, "memory-profile-file", "", "Path to the memory profile file.")

	opt.flags.IntVar(&opt.StatusUpdateMaxBatchSize, "status-update-max-batch-size", 20, "Number of object statuses to update at maximum in one transaction.")

	opt.viper.BindPFlags(opt.flags)

	return opt
}

// YAML returns yaml string of option, need to be called after calling Parse.
func (opt *Options) YAML() string {
	return opt.yamlStr
}

// UseInitialCluster returns true if the cluster.initial-cluster is defined. If it is, the ClusterJoinUrls is ignored.
func (opt *Options) UseInitialCluster() bool {
	return len(opt.Cluster.InitialCluster) > 0
}

// renameLegacyClusterRoles renames legacy writer/reader --> primary/secondary and raises warning.
func (opt *Options) renameLegacyClusterRoles() {
	warning := "Cluster roles writer/reader are deprecated. \n" +
		"Renamed cluster role '%s' to '%s'. Please use primary/secondary instead. \n"
	fmtLogger := fmt.Printf // Importing logger here is an import cycle, so use fmt instead.
	if opt.ClusterRole == "writer" {
		opt.ClusterRole = "primary"
		fmtLogger(warning, "writer", "primary")
	}
	if opt.ClusterRole == "reader" {
		opt.ClusterRole = "secondary"
		fmtLogger(warning, "reader", "secondary")
	}
}

// Parse parses all arguments, returns normal message without error if --help/--version set.
func (opt *Options) Parse() (string, error) {
	err := opt.flags.Parse(os.Args[1:])
	if err != nil {
		return "", err
	}

	if opt.ShowVersion {
		return version.Short, nil
	}

	if opt.ShowHelp {
		return opt.flags.FlagUsages(), nil
	}

	opt.viper.AutomaticEnv()
	opt.viper.SetEnvPrefix("EG")
	opt.viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if opt.ConfigFile != "" {
		opt.viper.SetConfigFile(opt.ConfigFile)
		opt.viper.SetConfigType("yaml")
		err := opt.viper.ReadInConfig()
		if err != nil {
			return "", fmt.Errorf("read config file %s failed: %v",
				opt.ConfigFile, err)
		}
	}

	// NOTE: Workaround because viper does not treat env vars the same as other config.
	// Reference: https://github.com/spf13/viper/issues/188#issuecomment-399518663
	for _, key := range opt.viper.AllKeys() {
		val := opt.viper.Get(key)
		// NOTE: We need to handle map[string]string
		// Reference: https://github.com/spf13/viper/issues/911
		if key == "labels" {
			val = opt.viper.GetStringMapString(key)
		}
		opt.viper.Set(key, val)
	}

	err = opt.viper.Unmarshal(opt, func(c *mapstructure.DecoderConfig) {
		c.TagName = "yaml"
	})
	if err != nil {
		return "", fmt.Errorf("yaml file unmarshal failed, please make sure you provide valid yaml file, %v", err)
	}

	opt.renameLegacyClusterRoles()
	err = opt.validate()
	if err != nil {
		return "", err
	}

	err = opt.prepare()
	if err != nil {
		return "", err
	}

	opt.adjust()

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

// adjust adjusts the options to handle conflict
// between user's config and internal component.
func (opt *Options) adjust() {
	if opt.ClusterRole != "primary" || opt.UseInitialCluster() {
		return
	}
	if len(opt.ClusterJoinURLs) == 0 {
		return
	}

	joinURL := opt.ClusterJoinURLs[0]

	for _, peerURL := range opt.ClusterInitialAdvertisePeerURLs {
		if strings.EqualFold(joinURL, peerURL) {
			fmt.Printf("cluster-join-urls %v changed to empty because it tries to join itself\n",
				opt.ClusterJoinURLs)
			// NOTE: We hack it this way to make sure the internal embedded etcd would
			// start a new cluster instead of joining existed one.
			opt.ClusterJoinURLs = nil
		}
	}
}

// ParseURLs parses list of strings to url.URL objects.
func ParseURLs(urlStrings []string) ([]url.URL, error) {
	urls := make([]url.URL, len(urlStrings))
	for i, urlString := range urlStrings {
		parsedURL, err := url.Parse(urlString)
		if err != nil {
			return nil, fmt.Errorf(" %s: %v", urlString, err)
		}
		urls[i] = *parsedURL
	}
	return urls, nil
}

// checkNoOverlappingArguments checks that only one of Cluster.InitialCluster and ClusterJoinURLs is defined.
func checkNoOverlappingArguments(opt *Options) error {
	if !opt.UseInitialCluster() {
		return nil
	}
	if len(opt.ClusterJoinURLs) == 0 {
		return nil
	}
	hasYAMLConfig := len(opt.ConfigFile) > 0
	if hasYAMLConfig {
		errorMsg := `cluster.initial-cluster and cluster-join-urls are both defined. ` +
			`Please provide only one of them. cluster.initial-cluster is the recommended way.`
		return fmt.Errorf(errorMsg)
	}
	errorMsg := `--initial-cluster and --cluster-join-urls are both defined. ` +
		`Please provide only one of them. --initial-cluster is the recommended way.`
	return fmt.Errorf(errorMsg)
}

func (opt *Options) validate() error {
	if opt.ClusterName == "" {
		return fmt.Errorf("empty cluster-name")
	} else if err := common.ValidateName(opt.ClusterName); err != nil {
		return err
	}
	if len(opt.ClusterJoinURLs) != 0 {
		if _, err := ParseURLs(opt.ClusterJoinURLs); err != nil {
			return fmt.Errorf("invalid cluster-join-urls: %v", err)
		}
	}
	switch opt.ClusterRole {
	case "secondary":
		if opt.ForceNewCluster {
			return fmt.Errorf("secondary got force-new-cluster")
		}
		if len(opt.Cluster.PrimaryListenPeerURLs) == 0 && len(opt.ClusterJoinURLs) == 0 {
			return fmt.Errorf("secondary got empty cluster.primary-listen-peer-urls and cluster-join-urls entries")
		}
	case "primary":
		if err := checkNoOverlappingArguments(opt); err != nil {
			return err
		}
		argumentsToValidate := map[string][]string{
			"cluster-listen-client-urls":          opt.ClusterListenClientURLs,
			"cluster-listen-peer-urls":            opt.ClusterListenPeerURLs,
			"cluster-advertise-client-urls":       opt.ClusterAdvertiseClientURLs,
			"cluster-initial-advertise-peer-urls": opt.ClusterInitialAdvertisePeerURLs,
		}

		if opt.UseInitialCluster() {
			argumentsToValidate = map[string][]string{
				"listen-client-urls":          opt.Cluster.ListenClientURLs,
				"listen-peer-urls":            opt.Cluster.ListenPeerURLs,
				"advertise-client-urls":       opt.Cluster.AdvertiseClientURLs,
				"initial-advertise-peer-urls": opt.Cluster.InitialAdvertisePeerURLs,
			}
			initialClusterUrls := make([]string, 0, len(opt.Cluster.InitialCluster))
			for _, value := range opt.Cluster.InitialCluster {
				initialClusterUrls = append(initialClusterUrls, value)
			}
			if _, err := ParseURLs(initialClusterUrls); err != nil {
				return fmt.Errorf("invalid initial-cluster: %v", err)
			}
		}
		for arg, urls := range argumentsToValidate {
			if len(urls) == 0 {
				return fmt.Errorf("empty %s", arg)
			}
			if _, err := ParseURLs(urls); err != nil {
				return fmt.Errorf("invalid %s: %v", arg, err)
			}
		}
	default:
		return fmt.Errorf("invalid cluster-role: supported roles are primary/secondary")
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
		return fmt.Errorf("empty home-dir")
	}
	if opt.DataDir == "" {
		return fmt.Errorf("empty data-dir")
	}
	if opt.LogDir == "" {
		return fmt.Errorf("empty log-dir")
	}
	if !opt.UseInitialCluster() && opt.MemberDir == "" {
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

// InitialClusterToString returns initial clusters string representation.
func (opt *Options) InitialClusterToString() string {
	ss := make([]string, 0)
	for name, peerURL := range opt.Cluster.InitialCluster {
		ss = append(ss, fmt.Sprintf("%s=%s", name, peerURL))
	}
	return strings.Join(ss, ",")
}

// GetPeerURLs returns URLs listed in cluster.initial-cluster for primary (a.k.a writer) and
// for secondary (a.k.a reader) the ones listed in cluster.primary-listen-peer-url.
func (opt *Options) GetPeerURLs() []string {
	if opt.ClusterRole == "secondary" {
		if len(opt.ClusterJoinURLs) != 0 {
			return opt.ClusterJoinURLs
		}
		return opt.Cluster.PrimaryListenPeerURLs
	}
	peerURLs := make([]string, 0)
	for _, peerURL := range opt.Cluster.InitialCluster {
		peerURLs = append(peerURLs, peerURL)
	}
	return peerURLs
}

// GetFirstAdvertiseClientURL returns the first advertised client url.
func (opt *Options) GetFirstAdvertiseClientURL() (string, error) {
	if opt.UseInitialCluster() {
		if len(opt.Cluster.AdvertiseClientURLs) == 0 {
			return "", fmt.Errorf("cluster.advertise-client-URLs is empty")
		}
		return opt.Cluster.AdvertiseClientURLs[0], nil
	}
	if len(opt.ClusterAdvertiseClientURLs) == 0 {
		return "", fmt.Errorf("cluster-advertise-client-URLs is empty")
	}
	return opt.ClusterAdvertiseClientURLs[0], nil
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
