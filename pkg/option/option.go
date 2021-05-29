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

	"github.com/megaease/easegress/pkg/common"
	"github.com/megaease/easegress/pkg/version"
	"github.com/mitchellh/mapstructure"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// Options is the startup options.
type Options struct {
	flags   *pflag.FlagSet
	viper   *viper.Viper
	yamlStr string

	// Flags from command line only.
	ShowVersion     bool   `yaml:"-"`
	ShowConfig      bool   `yaml:"-"`
	ConfigFile      string `yaml:"-"`
	ForceNewCluster bool   `yaml:"-"`

	// If a config file is specified, below command line flags will be ignored.

	// meta
	Name                  string            `yaml:"name"`
	Labels                map[string]string `yaml:"labels"`
	ClusterName           string            `yaml:"cluster-name"`
	ClusterRole           string            `yaml:"cluster-role"`
	ClusterRequestTimeout string            `yaml:"cluster-request-timeout"`
	ClusterClientURL      string            `yaml:"cluster-client-url"`
	ClusterPeerURL        string            `yaml:"cluster-peer-url"`
	ClusterJoinURLs       []string          `yaml:"cluster-join-urls"`
	APIAddr               string            `yaml:"api-addr"`
	Debug                 bool              `yaml:"debug"`

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
	opt.flags.BoolVarP(&opt.ShowConfig, "print-config", "c", false, "Print the configuration.")
	opt.flags.StringVarP(&opt.ConfigFile, "config-file", "f", "", "Load server configuration from a file(yaml format), other command line flags will be ignored if specified.")
	opt.flags.BoolVar(&opt.ForceNewCluster, "force-new-cluster", false, "Force to create a new one-member cluster.")
	opt.flags.StringVar(&opt.Name, "name", "eg-default-name", "Human-readable name for this member.")
	opt.flags.StringToStringVar(&opt.Labels, "labels", nil, "The labels for the instance of Easegress.")
	opt.flags.StringVar(&opt.ClusterName, "cluster-name", "eg-cluster-default-name", "Human-readable name for the new cluster, ignored while joining an existed cluster.")
	opt.flags.StringVar(&opt.ClusterRole, "cluster-role", "writer", "Cluster role for this member (reader, writer).")
	opt.flags.StringVar(&opt.ClusterRequestTimeout, "cluster-request-timeout", "10s", "Timeout to handle request in the cluster.")
	opt.flags.StringVar(&opt.ClusterClientURL, "cluster-client-url", "http://localhost:2379", "URL to listen on for cluster client traffic.")
	opt.flags.StringVar(&opt.ClusterPeerURL, "cluster-peer-url", "http://localhost:2380", "URL to listen on for cluster peer traffic.")
	opt.flags.StringSliceVar(&opt.ClusterJoinURLs, "cluster-join-urls", nil, "one or more urls of the writers in cluster to join, delimited by ',' without whitespaces.")
	opt.flags.StringVar(&opt.APIAddr, "api-addr", "localhost:2381", "Address([host]:port) to listen on for administration traffic.")
	opt.flags.BoolVar(&opt.Debug, "debug", false, "Flag to set lowest log level from INFO downgrade DEBUG.")

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
		if err == pflag.ErrHelp {
			return opt.flags.FlagUsages(), nil
		} else {
			return "", nil
		}
	}

	if opt.ShowVersion {
		return version.Short, nil
	}

	if opt.ConfigFile != "" {
		opt.viper.SetConfigFile(opt.ConfigFile)
		opt.viper.SetConfigType("yaml")
		err := opt.viper.ReadInConfig()
		if err != nil {
			return "", fmt.Errorf("read config file %s failed: %v",
				opt.ConfigFile, err)
		}
	}


	opt.viper.Unmarshal(opt, func(c *mapstructure.DecoderConfig) {
		c.TagName = "yaml"
	})

	opt.readEnv()
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

func (opt *Options) readEnv() {
	clientURL := os.Getenv("EG_CLUSTER_CLIENT_URL")
	if clientURL != "" {
		opt.ClusterClientURL = clientURL
	}

	peerURL := os.Getenv("EG_CLUSTER_PEER_URL")
	if peerURL != "" {
		opt.ClusterPeerURL = peerURL
	}
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

	if strings.EqualFold(joinURL, opt.ClusterPeerURL) {
		fmt.Printf("cluster-join-urls %v changed to empty because it tries to join itself",
			opt.ClusterJoinURLs)
		// NOTE: We hack it this way to make sure the internal embedded etcd would
		// start a new cluster instead of joining existed one.
		opt.ClusterJoinURLs = nil
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
		_, err := url.Parse(opt.ClusterPeerURL)
		if err != nil {
			return fmt.Errorf("invalid cluster-peer-url: %v", err)
		}
		_, err = url.Parse(opt.ClusterClientURL)
		if err != nil {
			return fmt.Errorf("invalid cluster-client-url: %v", err)
		}

		if len(opt.ClusterJoinURLs) != 0 {
			for _, urlText := range opt.ClusterJoinURLs {
				_, err := url.Parse(urlText)
				if err != nil {
					return fmt.Errorf("invalid cluster-join-urls: %v", err)
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
