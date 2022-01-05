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

package validator

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	lru "github.com/hashicorp/golang-lru"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

// BasicAuthValidatorSpec defines the configuration of Basic Auth validator.
// Only one of UserFile or UseEtcd should be defined.
type BasicAuthValidatorSpec struct {
	// UserFile is path to file containing encrypted user credentials in apache2-utils/htpasswd format.
	// To add user `userY`, use `sudo htpasswd /etc/apache2/.htpasswd userY`
	// Reference: https://manpages.debian.org/testing/apache2-utils/htpasswd.1.en.html#EXAMPLES
	UserFile string `yaml:"userFile" jsonschema:"omitempty"`
	// When UseEtcd is true, verify user credentials from etcd. Etcd stores them:
	// key: /credentials/{user id}
	// value: {encrypted password}
	UseEtcd bool `yaml:"useEtcd" jsonschema:"omitempty"`
}

// BasicAuthValidator defines the Basic Auth validator
type BasicAuthValidator struct {
	spec                 *BasicAuthValidatorSpec
	authorizedUsersCache *lru.Cache
	cluster              cluster.Cluster
}

func parseCredentials(creds string) (string, string, error) {
	parts := strings.Split(creds, ":")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("bad format")
	}
	return parts[0], parts[1], nil
}

func (bav *BasicAuthValidator) getFromFile(targetUserID string) (string, error) {
	file, err := os.OpenFile(bav.spec.UserFile, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("open file error: %v", err)
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := sc.Text()
		userID, password, err := parseCredentials(line)
		if err != nil {
			return "", err
		}
		if userID == targetUserID {
			return password, nil
		}
	}
	return "", fmt.Errorf("unauthorized")
}

func (bav *BasicAuthValidator) getFromEtcd(targetUserID string) (string, error) {
	const prefix = "/credentials/"
	password, err := bav.cluster.Get(prefix + targetUserID)
	if err != nil {
		return "", err
	}
	if password == nil {
		return "", fmt.Errorf("unauthorized")
	}
	return *password, nil
}

// NewBasicAuthValidator creates a new Basic Auth validator
func NewBasicAuthValidator(spec *BasicAuthValidatorSpec, supervisor *supervisor.Supervisor) *BasicAuthValidator {
	var cluster cluster.Cluster
	if spec.UseEtcd {
		if supervisor == nil || supervisor.Cluster() == nil {
			logger.Errorf("BasicAuth validator : failed to read data from etcd")
		} else {
			cluster = supervisor.Cluster()
		}
	}
	cache, err := lru.New(256)
	if err != nil {
		panic(err)
	}
	bav := &BasicAuthValidator{
		spec:                 spec,
		authorizedUsersCache: cache,
		cluster:              cluster,
	}
	return bav
}

func (bav *BasicAuthValidator) getUserFromCache(userID string) (string, bool) {
	if val, ok := bav.authorizedUsersCache.Get(userID); ok {
		return val.(string), true
	}
	var password string
	var err error
	if bav.spec.UserFile != "" {
		password, err = bav.getFromFile(userID)
	}
	if bav.spec.UseEtcd == true {
		password, err = bav.getFromEtcd(userID)
	}
	if err != nil {
		return "", false
	}
	bav.authorizedUsersCache.Add(userID, password)
	return password, true
}

// Validate validates the Authorization header of a http request
func (bav *BasicAuthValidator) Validate(req context.HTTPRequest) error {
	hdr := req.Header()
	base64credentials, err := parseAuthorizationHeader(hdr)
	if err != nil {
		return err
	}
	credentialBytes, err := base64.StdEncoding.DecodeString(base64credentials)
	if err != nil {
		return fmt.Errorf("error occured during base64 decode: %s", err.Error())
	}
	credentials := string(credentialBytes)
	userID, token, err := parseCredentials(credentials)
	if err != nil {
		return fmt.Errorf("unauthorized")
	}

	if expectedToken, ok := bav.getUserFromCache(userID); ok && expectedToken == token {
		return nil
	}
	return fmt.Errorf("unauthorized")
}
