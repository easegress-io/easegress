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

// Options is the startup options.
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
	Name                            string            `yaml:"name" env:"EG_NAME"`
	Labels                          map[string]string `yaml:"labels" env:"EG_LABELS"`
	ClusterName                     string            `yaml:"cluster-name"`
	ClusterRole                     string            `yaml:"cluster-role"`
	ClusterRequestTimeout           string            `yaml:"cluster-request-timeout"`
	ClusterListenClientURLs         []string          `yaml:"cluster-listen-client-urls"`
	ClusterListenPeerURLs           []string          `yaml:"cluster-listen-peer-urls"`
	ClusterAdvertiseClientURLs      []string          `yaml:"cluster-advertise-client-urls"`
	ClusterInitialAdvertisePeerURLs []string          `yaml:"cluster-initial-advertise-peer-urls"`
	ClusterJoinURLs                 []string          `yaml:"cluster-join-urls"`
	APIAddr                         string            `yaml:"api-addr"`
	Debug                           bool              `yaml:"debug"`
	InitialObjectConfigFiles        []string          `yaml:"initial-object-config-files"`

	// Path.
	HomeDir   string `yaml:"home-dir"`
	DataDir   string `yaml:"data-dir"`
	WALDir    string `yaml:"wal-dir"`
	LogDir    string `yaml:"log-dir"`
	MemberDir string `yaml:"member-dir"`

	// Profile.
	CPUProfileFile    string `yaml:"cpu-profile-file"`
	MemoryProfileFile string `yaml:"memory-profile-file"`

	// Prepare the items below in advance.
	AbsHomeDir   string `yaml:"-"`
	AbsDataDir   string `yaml:"-"`
	AbsWALDir    string `yaml:"-"`
	AbsLogDir    string `yaml:"-"`
	AbsMemberDir string `yaml:"-"`
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
	opt.flags.StringVar(&opt.ClusterName, "cluster-name", "eg-cluster-default-name", "Human-readable name for the new cluster, ignored while joining an existed cluster.")
	opt.flags.StringVar(&opt.ClusterRole, "cluster-role", "writer", "Cluster role for this member (reader, writer).")
	opt.flags.StringVar(&opt.ClusterRequestTimeout, "cluster-request-timeout", "10s", "Timeout to handle request in the cluster.")
	opt.flags.StringSliceVar(&opt.ClusterListenClientURLs, "cluster-listen-client-urls", []string{"http://localhost:2379"}, "List of URLs to listen on for cluster client traffic.")
	opt.flags.StringSliceVar(&opt.ClusterListenPeerURLs, "cluster-listen-peer-urls", []string{"http://localhost:2380"}, "List of URLs to listen on for cluster peer traffic.")
	opt.flags.StringSliceVar(&opt.ClusterAdvertiseClientURLs, "cluster-advertise-client-urls", []string{"http://localhost:2379"}, "List of this member’s client URLs to advertise to the rest of the cluster.")
	opt.flags.StringSliceVar(&opt.ClusterInitialAdvertisePeerURLs, "cluster-initial-advertise-peer-urls", []string{"http://localhost:2380"}, "List of this member’s peer URLs to advertise to the rest of the cluster.")
	opt.flags.StringSliceVar(&opt.ClusterJoinURLs, "cluster-join-urls", nil, "List of URLs to join, when the first url is the same with any one of cluster-initial-advertise-peer-urls, it means to join itself, and this config will be treated empty.")
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

	opt.viper.BindPFlags(opt.flags)

	return opt
}

// YAML returns yaml string of option, need to be called after calling Parse.
func (opt *Options) YAML() string {
	return opt.yamlStr
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

	opt.viper.Unmarshal(opt, func(c *mapstructure.DecoderConfig) {
		c.TagName = "yaml"
	})

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
	if opt.ClusterRole != "writer" {
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

		if len(opt.ClusterJoinURLs) == 0 {
			return fmt.Errorf("reader got empty cluster-join-urls")
		}

		for _, urlText := range opt.ClusterJoinURLs {
			_, err := url.Parse(urlText)
			if err != nil {
				return fmt.Errorf("invalid cluster-join-urls: %v", err)
			}
		}

	case "writer":
		if len(opt.ClusterListenClientURLs) == 0 {
			return fmt.Errorf("empty cluster-listen-client-urls")
		}
		if len(opt.ClusterListenPeerURLs) == 0 {
			return fmt.Errorf("empty cluster-listen-peer-urls")
		}
		if len(opt.ClusterAdvertiseClientURLs) == 0 {
			return fmt.Errorf("empty cluster-advertise-client-urls")
		}
		if len(opt.ClusterInitialAdvertisePeerURLs) == 0 {
			return fmt.Errorf("empty cluster-initial-advertise-peer-urls")
		}

		for _, clientURL := range opt.ClusterListenClientURLs {
			_, err := url.Parse(clientURL)
			if err != nil {
				return fmt.Errorf("invalid cluster-listen-client-urls: %s: %v", clientURL, err)
			}
		}
		for _, peerURL := range opt.ClusterListenPeerURLs {
			_, err := url.Parse(peerURL)
			if err != nil {
				return fmt.Errorf("invalid cluster-listen-peer-urls: %s: %v", peerURL, err)
			}
		}
		for _, clientURL := range opt.ClusterAdvertiseClientURLs {
			_, err := url.Parse(clientURL)
			if err != nil {
				return fmt.Errorf("invalid cluster-advertise-client-urls: %s: %v", clientURL, err)
			}
		}
		for _, peerURL := range opt.ClusterInitialAdvertisePeerURLs {
			_, err := url.Parse(peerURL)
			if err != nil {
				return fmt.Errorf("invalid cluster-initial-advertise-peer-urls: %s: %v", peerURL, err)
			}
		}

		if len(opt.ClusterJoinURLs) != 0 {
			for _, joinURL := range opt.ClusterJoinURLs {
				_, err := url.Parse(joinURL)
				if err != nil {
					return fmt.Errorf("invalid cluster-join-urls: %s: %v", joinURL, err)
				}
			}
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
		return fmt.Errorf("empty home-dir")
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
