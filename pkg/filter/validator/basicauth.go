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
	"os"
	"bufio"
	"fmt"
	"encoding/base64"
	"strings"

	"github.com/megaease/easegress/pkg/context"
)

// BasicAuthValidatorSpec defines the configuration of Basic Auth validator.
// Only one of UserFile or UseEtcd should be defined.
type BasicAuthValidatorSpec struct {
	// UserFile is path to file containing encrypted user credentials in apache2-utils/htpasswd format.
	// To add user `userY`, use `sudo htpasswd /etc/apache2/.htpasswd userY`
	// Reference: https://manpages.debian.org/testing/apache2-utils/htpasswd.1.en.html#EXAMPLES
	UserFile string `yaml:"userFile" jsonschema:"omitempty"`
	// When UseEtcd is true, verify user credentials from etcd.
	// TODO add some details about this.
	UseEtcd bool `yaml:"useEtcd" jsonschema:"omitempty"`
}

type AuthorizedUsers struct {
	nameToToken	map[string]string
}

func NewAuthorizedUsers() *AuthorizedUsers {
	return &AuthorizedUsers{
		nameToToken: make(map[string]string),
	}
}

func (au *AuthorizedUsers) Validate(user string, token string) error {
	if expectedToken, ok := au.nameToToken[user]; ok && expectedToken == token {
		return nil
	}
	return fmt.Errorf("unauthorized")
}

func parseCredentials(creds string) (string, string, error) {
	parts := strings.Split(creds, ":")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("bad format")
	}
	return parts[0], parts[1], nil
}

func (au *AuthorizedUsers) UpdateFromUserFile(fileName string) error {
	file, err := os.OpenFile(fileName, os.O_RDONLY, os.ModePerm)
    if err != nil {
        return fmt.Errorf("open file error: %v", err)
    }
    defer file.Close()
	sc := bufio.NewScanner(file)
    for sc.Scan() {
        line := sc.Text()
		name, token, err := parseCredentials(line)
		if err != nil {
			return err
		}
		au.nameToToken[name] = token
    }
	return nil
}

// NewBasicAuthValidator creates a new Basic Auth validator
func NewBasicAuthValidator(spec *BasicAuthValidatorSpec) *BasicAuthValidator {
	au := NewAuthorizedUsers()
	if spec.UserFile != "" {
		au.UpdateFromUserFile(spec.UserFile)
	}
	return &BasicAuthValidator{
		spec:        spec,
		authorizedUsers: au,
	}
}

// BasicAuthValidator defines the Basic Auth validator
type BasicAuthValidator struct {
	spec        *BasicAuthValidatorSpec
	authorizedUsers		*AuthorizedUsers
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
	name, token, err := parseCredentials(credentials)
	if err != nil {
		return fmt.Errorf("unauthorized")
	}
	return bav.authorizedUsers.Validate(name, token)
}
