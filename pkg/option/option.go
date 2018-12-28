package option

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/version"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"

	flags "github.com/jessevdk/go-flags"
)

var (
	Name     string
	Mode     string
	PeerURLs []string
	APIURL   string

	StoreDir string

	LogDir      string
	StdLogLevel string

	CPUProfileFile    string
	MemoryProfileFile string

	PipelineInitParallelism uint32
	PipelineMinParallelism  uint32
	PipelineMaxParallelism  uint32

	PluginIODataFormatLengthLimit uint64
	PluginPythonRootNamespace     bool
	PluginShellRootNamespace      bool
	CGIDir                        string
	CertDir                       string
)

func setGlobalFlag(s *server) {
	Name = s.Name
	Mode = s.Mode
	PeerURLs = s.PeerURLs
	APIURL = s.APIURL

	StoreDir = s.StoreDir

	LogDir = s.LogDir
	StdLogLevel = s.StdLogLevel

	CPUProfileFile = s.CPUProfileFile
	MemoryProfileFile = s.MemoryProfileFile

	PipelineInitParallelism = s.PipelineInitParallelism
	PipelineMinParallelism = s.PipelineMinParallelism
	PipelineMaxParallelism = s.PipelineMaxParallelism

	PluginIODataFormatLengthLimit = s.PluginIODataFormatLengthLimit
	PluginPythonRootNamespace = s.PluginPythonRootNamespace
	PluginShellRootNamespace = s.PluginShellRootNamespace
	CGIDir = s.CGIDir
	CertDir = s.CertDir
}

type flagGroup interface {
	validate() error
}

type meta struct {
	Name     string   `yaml:"name" long:"name" description:"Human-readable name for this member."`
	Mode     string   `yaml:"mode" long:"mode" description:"Cluster mode for this member.(write, read)"`
	APIURL   string   `yaml:"api-url" long:"api-url" description:"URL to listen on for client traffic."`
	PeerURLs []string `yaml:"peer-urls" long:"peer-urls" description:"List of URLs to listen on for peer traffic"`
}

func (m meta) validate() error {
	if m.Name == "" {
		return fmt.Errorf("empty name")
	}

	switch strings.ToLower(m.Mode) {
	case "read", "write":
	default:
		return fmt.Errorf("invalid mode(support write, read)")
	}

	if m.APIURL == "" {
		return fmt.Errorf("empty api-url")
	}

	return nil
}

type store struct {
	StoreDir string `yaml:"store-dir" long:"store-dir" description:"Path to the store directory."`
}

func (s store) validate() error {
	if s.StoreDir == "" {
		return fmt.Errorf("empty store-dir")
	}

	return nil
}

type log struct {
	LogDir      string `yaml:"log-dir" long:"log-dir" description:"Path to the log directory."`
	StdLogLevel string `yaml:"std-log-level" long:"std-log-level" description:"Set standard log level"`
}

func (l log) validate() error {
	if _, err := logrus.ParseLevel(l.StdLogLevel); err != nil {
		return fmt.Errorf("invalid std-log-level: %v", err)
	}
	if l.LogDir == "" {
		return fmt.Errorf("empty log-dir")
	}

	return nil
}

type profile struct {
	CPUProfileFile    string `yaml:"cpu-profile-file" long:"cpu-profile-file" description:"Path to the CPU profile file."`
	MemoryProfileFile string `yaml:"memory-profile-file" long:"memory-profile-file" description:"Path to the memory profile file."`
}

func (p profile) validate() error {
	return nil
}

type pipeline struct {
	PipelineInitParallelism uint32 `yaml:"pipeline-init-parallelism" long:"pipeline-init-parallelism" description:"Initial parallelism for a pipeline running in dynamic schedule mode."`
	PipelineMinParallelism  uint32 `yaml:"pipeline-min-parallelism" long:"pipeline-min-parallelism" description:"Minimum parallelism for a pipeline running in dynamic schedule mode."`
	PipelineMaxParallelism  uint32 `yaml:"pipeline-max-parallelism" long:"pipeline-max-parallelism" description:"Maximum parallelism for a pipeline running in dynamic schedule mode."`
}

func (p pipeline) validate() error {
	if p.PipelineInitParallelism < 1 ||
		p.PipelineInitParallelism > uint32(^uint16(0)) {
		return fmt.Errorf("invalid pipeline-init-parallelism[1,%d]", ^uint16(0))
	}

	if p.PipelineMaxParallelism < 1 ||
		p.PipelineMaxParallelism > 102400 {
		return fmt.Errorf("incalid pipeline-max-parallelism[1,102400]")
	}

	if p.PipelineMinParallelism > p.PipelineMaxParallelism {
		return fmt.Errorf("pipeline-min-parallelism %d > pipeline-max-parallelism %d",
			p.PipelineMinParallelism, p.PipelineMaxParallelism)
	}

	return nil
}

type plugin struct {
	PluginIODataFormatLengthLimit uint64 `yaml:"plugin-io-data-format-len-limit" long:"plugin-io-data-format-len-limit" description:"Bytes limitation on plugin IO data formation output."`
	PluginPythonRootNamespace     bool   `yaml:"plugin-python-root-namespace" long:"plugin-python-root-namespace" description:"Run python code in root namespace without isolation."`
	PluginShellRootNamespace      bool   `yaml:"plugin-shell-root-namespace" long:"plugin-shell-root-namespace" description:"Run shell code in root namespace without isolation."`

	CGIDir  string `yaml:"cgi-dir" long:"cgi-dir" description:"Path to the CGI directory."`
	CertDir string `yaml:"cert-dir" long:"cert-dir" description:"Path to the Certificate directory."`
}

func (p plugin) validate() error {
	if p.PluginIODataFormatLengthLimit < 1 ||
		p.PluginIODataFormatLengthLimit > 10000009 {
		return fmt.Errorf("invalid plugin-io-data-format-len-limit[1,10000009]")
	}

	return nil
}

type server struct {
	ShowVersion bool   `yaml:"-" short:"v" long:"version" description:"Print the version and exit."`
	ShowConfig  bool   `yaml:"-" short:"c" long:"print-config" description:"Print the configuration."`
	ConfigFile  string `yaml:"-" short:"f" long:"config-file" description:"Load server configuration from a file(yaml format), other command line flags will be ignored if specified."`

	// If a config file is specified, below command line flags will be ignored.
	meta     `yaml:",inline"`
	store    `yaml:",inline"`
	profile  `yaml:",inline"`
	log      `yaml:",inline"`
	pipeline `yaml:",inline"`
	plugin   `yaml:",inline"`
}

func (s *server) validate() error {
	flagGroups := []flagGroup{s.meta, s.store, s.profile, s.log, s.pipeline, s.plugin}
	for _, fg := range flagGroups {
		if err := fg.validate(); err != nil {
			return err
		}
	}

	return nil
}

func init() {
	// Set default value in one place.
	s := &server{
		meta: meta{
			Name:     "member-001",
			Mode:     "write",
			APIURL:   "localhost:8080",
			PeerURLs: nil,
		},
		store: store{
			StoreDir: "./store",
		},
		log: log{
			LogDir:      "./logs",
			StdLogLevel: "INFO",
		},
		pipeline: pipeline{
			PipelineInitParallelism: 1,
			PipelineMinParallelism:  5,
			PipelineMaxParallelism:  5120,
		},
		plugin: plugin{
			PluginIODataFormatLengthLimit: 128,
			PluginPythonRootNamespace:     false,
			PluginShellRootNamespace:      false,

			CGIDir:  "./cgi",
			CertDir: "./cert",
		},
	}

	_, err := flags.NewParser(s, flags.HelpFlag|flags.PassDoubleDash).Parse()
	if err != nil {
		if err, ok := err.(*flags.Error); ok && err.Type == flags.ErrHelp {
			common.Exit(0, err.Message)
		}
		common.Exit(1, err.Error())
	}

	if s.ShowVersion {
		common.Exit(0, version.Short)
	}

	if s.ConfigFile != "" {
		buff, err := ioutil.ReadFile(s.ConfigFile)
		if err != nil {
			common.Exit(1, fmt.Sprintf("read config file failed: %v", err))
		}
		err = yaml.Unmarshal(buff, s)
		if err != nil {
			common.Exit(1, fmt.Sprintf("unmarshal config file %s to yaml failed: %v",
				s.ConfigFile, err))
		}
	}

	if s.ShowConfig {
		buff, err := yaml.Marshal(s)
		if err != nil {
			common.Exit(1, fmt.Sprintf("marshal config file %s to yaml failed: %v",
				s.ConfigFile, err))
		}
		fmt.Printf("%s", buff)
	}

	if err := s.validate(); err != nil {
		common.Exit(1, err.Error())
	}

	setGlobalFlag(s)
}
