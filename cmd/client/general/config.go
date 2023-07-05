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

package general

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/megaease/easegress/pkg/util/codectool"
)

type Config struct {
	Kind           string          `json:"kind"`
	Clusters       []NamedCluster  `json:"clusters"`
	AuthInfos      []NamedAuthInfo `json:"users"`
	Contexts       []NamedContext  `json:"contexts"`
	CurrentContext string          `json:"current-context"`
}

type Cluster struct {
	Server                   string `json:"server"`
	CertificateAuthority     string `json:"certificate-authority,omitempty"`
	CertificateAuthorityData []byte `json:"certificate-authority-data,omitempty"`
}

type AuthInfo struct {
	ClientCertificate     string `json:"client-certificate,omitempty"`
	ClientCertificateData []byte `json:"client-certificate-data,omitempty"`
	ClientKey             string `json:"client-key,omitempty"`
	ClientKeyData         []byte `json:"client-key-data,omitempty"`
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
}

type AuthInfoMarshaler struct {
	ClientCertificateData string `json:"client-certificate-data,omitempty"`
	ClientKeyData         string `json:"client-key-data,omitempty"`
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
}

type Context struct {
	Cluster  string `json:"cluster"`
	AuthInfo string `json:"user"`
}

type NamedCluster struct {
	Name    string  `json:"name"`
	Cluster Cluster `json:"cluster"`
}

type NamedContext struct {
	Name    string  `json:"name"`
	Context Context `json:"context"`
}

type NamedAuthInfo struct {
	Name     string   `json:"name"`
	AuthInfo AuthInfo `json:"user"`
}

type CurrentConfig struct {
	CurrentContext string        `json:"current-context"`
	Context        NamedContext  `json:"context"`
	Cluster        NamedCluster  `json:"cluster"`
	AuthInfo       NamedAuthInfo `json:"user"`
}

func (c *CurrentConfig) GetServer() string {
	return c.Cluster.Cluster.Server
}

func (c *CurrentConfig) GetUsername() string {
	return c.AuthInfo.AuthInfo.Username
}

func (c *CurrentConfig) GetPassword() string {
	return c.AuthInfo.AuthInfo.Password
}

func (c *CurrentConfig) GetClientCertificateData() []byte {
	return c.AuthInfo.AuthInfo.ClientCertificateData
}

func (c *CurrentConfig) GetClientCertificate() string {
	return c.AuthInfo.AuthInfo.ClientCertificate
}
func (c *CurrentConfig) GetClientKey() string {
	return c.AuthInfo.AuthInfo.ClientKey
}

func (c *CurrentConfig) GetClientKeyData() []byte {
	return c.AuthInfo.AuthInfo.ClientKeyData
}

func (c *CurrentConfig) GetCertificateAuthority() string {
	return c.Cluster.Cluster.CertificateAuthority
}

func (c *CurrentConfig) GetCertificateAuthorityData() []byte {
	return c.Cluster.Cluster.CertificateAuthorityData
}

func (c *CurrentConfig) UseHTTPS() bool {
	return len(c.GetCertificateAuthorityData()) > 0 || len(c.GetCertificateAuthority()) > 0 || len(c.GetClientCertificateData()) > 0 || len(c.GetClientCertificate()) > 0
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return path.Join(homeDir, ".egctlconfig"), nil
}

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

func GetRedactedConfig(c *Config) *Config {
	copy := *c
	redacted, _ := base64.StdEncoding.DecodeString(string("REDACTED"))
	copy.Clusters = Map(copy.Clusters, func(c NamedCluster) NamedCluster {
		if len(c.Cluster.CertificateAuthorityData) > 0 {
			c.Cluster.CertificateAuthorityData = redacted
		}
		return c
	})
	copy.AuthInfos = Map(copy.AuthInfos, func(u NamedAuthInfo) NamedAuthInfo {
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
	return &copy
}

var globalCurrentConfig *CurrentConfig

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
		return nil, fmt.Errorf("current context not found in .egctlconfig file")
	}

	currentCtx, ok := Find(config.Contexts, func(c NamedContext) bool {
		return c.Name == config.CurrentContext
	})
	if !ok {
		return nil, fmt.Errorf("current context %s not found in .egctlconfig file", config.CurrentContext)
	}
	cluster, ok := Find(config.Clusters, func(c NamedCluster) bool {
		return c.Name == currentCtx.Context.Cluster
	})
	if !ok {
		return nil, fmt.Errorf("cluster %s not found in .egctlconfig file", currentCtx.Context.Cluster)
	}
	user, ok := Find(config.AuthInfos, func(u NamedAuthInfo) bool {
		return u.Name == currentCtx.Context.AuthInfo
	})
	if !ok {
		return nil, fmt.Errorf("user %s not found in .egctlconfig file", currentCtx.Context.AuthInfo)
	}
	globalCurrentConfig = &CurrentConfig{
		CurrentContext: config.CurrentContext,
		Context:        *currentCtx,
		Cluster:        *cluster,
		AuthInfo:       *user,
	}
	return globalCurrentConfig, nil
}
