/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package general provides the general utilities for the client.
package general

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

type (
	// GlobalFlags is the global flags for the whole client.
	GlobalFlags struct {
		Server             string
		ForceTLS           bool
		InsecureSkipVerify bool
		OutputFormat       string

		// following are some general flags. Can be used by all commands. But not all commands use them.
		Verbose bool
	}

	// APIErr is the standard return of error.
	APIErr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

// DefaultFormat is the default output format.
func (g *GlobalFlags) DefaultFormat() bool {
	return g.OutputFormat == DefaultFormat
}

// CmdGlobalFlags is the singleton of GlobalFlags.
var CmdGlobalFlags GlobalFlags

/*
clusters:
  - cluster:
      server: localhost:2381
      certificate-authority: "/tmp/certs/ca.crt"
	  certificate-authority-data: "xxxx"
    name: cluster-default
contexts:
  - context:
      cluster: cluster-default
      user: user-default
    name: context-default
current-context: context-default
kind: Config
users:
  - name: user-default
    user:
      username: admin
      password: admin
      client-certificate-data: "xxx"
      client-key-data: "xxx"
      client-certificate: "/tmp/certs/client.crt"
      client-key: "/tmp/certs/client.key"
*/

// Config is the configuration of egctl.
type Config struct {
	Kind           string          `json:"kind"`
	Clusters       []NamedCluster  `json:"clusters"`
	AuthInfos      []NamedAuthInfo `json:"users"`
	Contexts       []NamedContext  `json:"contexts"`
	CurrentContext string          `json:"current-context"`
}

// Cluster is the cluster configuration.
type Cluster struct {
	Server                   string `json:"server"`
	CertificateAuthority     string `json:"certificate-authority,omitempty"`
	CertificateAuthorityData []byte `json:"certificate-authority-data,omitempty"`
}

// AuthInfo is the user configuration.
type AuthInfo struct {
	ClientCertificate     string `json:"client-certificate,omitempty"`
	ClientCertificateData []byte `json:"client-certificate-data,omitempty"`
	ClientKey             string `json:"client-key,omitempty"`
	ClientKeyData         []byte `json:"client-key-data,omitempty"`
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
}

// Context is the context configuration.
type Context struct {
	Cluster  string `json:"cluster"`
	AuthInfo string `json:"user"`
}

// NamedCluster is the cluster with name.
type NamedCluster struct {
	Name    string  `json:"name"`
	Cluster Cluster `json:"cluster"`
}

// NamedContext is the context with name.
type NamedContext struct {
	Name    string  `json:"name"`
	Context Context `json:"context"`
}

// NamedAuthInfo is the user with name.
type NamedAuthInfo struct {
	Name     string   `json:"name"`
	AuthInfo AuthInfo `json:"user"`
}

// CurrentConfig is config contains current used cluster and user.
type CurrentConfig struct {
	CurrentContext string        `json:"current-context"`
	Context        NamedContext  `json:"context"`
	Cluster        NamedCluster  `json:"cluster"`
	AuthInfo       NamedAuthInfo `json:"user"`
}

// GetServer returns the current used server.
func (c *CurrentConfig) GetServer() string {
	return c.Cluster.Cluster.Server
}

// GetUsername returns the current used username.
func (c *CurrentConfig) GetUsername() string {
	return c.AuthInfo.AuthInfo.Username
}

// GetPassword returns the current used password.
func (c *CurrentConfig) GetPassword() string {
	return c.AuthInfo.AuthInfo.Password
}

// GetClientCertificateData returns the current used client certificate data.
func (c *CurrentConfig) GetClientCertificateData() []byte {
	return c.AuthInfo.AuthInfo.ClientCertificateData
}

// GetClientCertificate returns the current used client certificate file name.
func (c *CurrentConfig) GetClientCertificate() string {
	return c.AuthInfo.AuthInfo.ClientCertificate
}

// GetClientKey returns the current used client key file name.
func (c *CurrentConfig) GetClientKey() string {
	return c.AuthInfo.AuthInfo.ClientKey
}

// GetClientKeyData returns the current used client key data.
func (c *CurrentConfig) GetClientKeyData() []byte {
	return c.AuthInfo.AuthInfo.ClientKeyData
}

// GetCertificateAuthority returns the current used certificate authority file name.
func (c *CurrentConfig) GetCertificateAuthority() string {
	return c.Cluster.Cluster.CertificateAuthority
}

// GetCertificateAuthorityData returns the current used certificate authority data.
func (c *CurrentConfig) GetCertificateAuthorityData() []byte {
	return c.Cluster.Cluster.CertificateAuthorityData
}

// UseHTTPS returns whether the current used server is HTTPS.
func (c *CurrentConfig) UseHTTPS() bool {
	return len(c.GetCertificateAuthorityData()) > 0 || len(c.GetCertificateAuthority()) > 0 || len(c.GetClientCertificateData()) > 0 || len(c.GetClientCertificate()) > 0
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return path.Join(homeDir, ".egctlrc"), nil
}

// WriteConfig writes the config to file.
func WriteConfig(config *Config) error {
	data, err := codectool.MarshalJSON(config)
	if err != nil {
		return err
	}
	data, err = codectool.JSONToYAML(data)
	if err != nil {
		return err
	}

	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	info, err := os.Stat(configPath)
	if err != nil {
		return err
	}

	err = os.WriteFile(configPath, data, info.Mode())
	if err != nil {
		return err
	}

	return nil
}

var globalConfig *Config

// GetConfig returns the config.
func GetConfig() (*Config, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(configPath)
	if errors.Is(err, os.ErrNotExist) {
		// not exist means no config
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = codectool.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("unmarshal config file %s failed: %v", configPath, err)
	}
	globalConfig = config
	return config, nil
}

// GetRedactedConfig returns the config with sensitive data redacted.
func GetRedactedConfig(c *Config) *Config {
	config := *c
	redacted, _ := base64.StdEncoding.DecodeString(string("REDACTED"))
	config.Clusters = Map(config.Clusters, func(c NamedCluster) NamedCluster {
		if len(c.Cluster.CertificateAuthorityData) > 0 {
			c.Cluster.CertificateAuthorityData = redacted
		}
		return c
	})
	config.AuthInfos = Map(config.AuthInfos, func(u NamedAuthInfo) NamedAuthInfo {
		if len(u.AuthInfo.ClientCertificateData) > 0 {
			u.AuthInfo.ClientCertificateData = redacted
		}
		if len(u.AuthInfo.ClientKeyData) > 0 {
			u.AuthInfo.ClientKeyData = redacted
		}
		if len(u.AuthInfo.Password) > 0 {
			u.AuthInfo.Password = "REDACTED"
		}
		return u
	})
	return &config
}

var globalCurrentConfig *CurrentConfig

// GetCurrentConfig returns the current config.
func GetCurrentConfig() (*CurrentConfig, error) {
	if globalCurrentConfig != nil {
		return globalCurrentConfig, nil
	}

	config, err := GetConfig()
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, nil
	}
	if config.CurrentContext == "" {
		return nil, fmt.Errorf("current context not found in .egctlrc file")
	}

	currentCtx, ok := Find(config.Contexts, func(c NamedContext) bool {
		return c.Name == config.CurrentContext
	})
	if !ok {
		return nil, fmt.Errorf("current context %s not found in .egctlrc file", config.CurrentContext)
	}
	cluster, ok := Find(config.Clusters, func(c NamedCluster) bool {
		return c.Name == currentCtx.Context.Cluster
	})
	if !ok {
		return nil, fmt.Errorf("cluster %s not found in .egctlrc file", currentCtx.Context.Cluster)
	}
	user, ok := Find(config.AuthInfos, func(u NamedAuthInfo) bool {
		return u.Name == currentCtx.Context.AuthInfo
	})
	if !ok {
		return nil, fmt.Errorf("user %s not found in .egctlrc file", currentCtx.Context.AuthInfo)
	}
	globalCurrentConfig = &CurrentConfig{
		CurrentContext: config.CurrentContext,
		Context:        *currentCtx,
		Cluster:        *cluster,
		AuthInfo:       *user,
	}
	return globalCurrentConfig, nil
}
