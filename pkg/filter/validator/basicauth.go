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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tg123/go-htpasswd"
	"golang.org/x/crypto/bcrypt"

	"github.com/megaease/easegress/pkg/cluster"
	httpcontext "github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

type (
	// EtcdSpec defines etcd prefix and which yaml entries are the username and password. For example spec
	//   prefix: "/creds/"
	//   usernameKey: "user"
	//   passwordKey: "pw"
	// expects the yaml to be stored with key /custom-data/creds/{id} in following yaml (extra keys are allowed)
	//   user: doge
	//   pw: {encrypted or plain text password}
	EtcdSpec struct {
		Prefix      string `yaml:"prefix" jsonschema:"onitempty"`
		UsernameKey string `yaml:"usernameKey" jsonschema:"omitempty"`
		PasswordKey string `yaml:"passwordKey" jsonschema:"omitempty"`
	}
	// BasicAuthValidatorSpec defines the configuration of Basic Auth validator.
	// Only one of UserFile or Etcd should be defined.
	BasicAuthValidatorSpec struct {
		// UserFile is path to file containing encrypted user credentials in apache2-utils/htpasswd format.
		// To add user `userY`, use `sudo htpasswd /etc/apache2/.htpasswd userY`
		// Reference: https://manpages.debian.org/testing/apache2-utils/htpasswd.1.en.html#EXAMPLES
		UserFile string `yaml:"userFile" jsonschema:"omitempty"`
		// When etcd is specified, verify user credentials from etcd. Etcd stores them:
		// key: /custom-data/{etcd.prefix}/{username}
		// value: {yaml string in format of etcd}
		Etcd *EtcdSpec `yaml:"etcd" jsonschema:"omitempty"`
	}

	// AuthorizedUsersCache provides cached lookup for authorized users.
	AuthorizedUsersCache interface {
		Match(string, string) bool
		WatchChanges() error
		Refresh() error
		Close()
	}

	htpasswdUserCache struct {
		userFile       string
		userFileObject *htpasswd.File
		fileMutex      sync.RWMutex
		syncInterval   time.Duration
		stopCtx        context.Context
		cancel         context.CancelFunc
	}

	etcdUserCache struct {
		userFileObject *htpasswd.File
		cluster        cluster.Cluster
		prefix         string
		usernameKey    string
		passwordKey    string
		syncInterval   time.Duration
		stopCtx        context.Context
		cancel         context.CancelFunc
	}

	// BasicAuthValidator defines the Basic Auth validator
	BasicAuthValidator struct {
		spec                 *BasicAuthValidatorSpec
		authorizedUsersCache AuthorizedUsersCache
	}
)

const customDataPrefix = "/custom-data/"

func parseCredentials(creds string) (string, string, error) {
	parts := strings.Split(creds, ":")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("bad format")
	}
	return parts[0], parts[1], nil
}

func bcryptHash(data []byte) (string, error) {
	pw, err := bcrypt.GenerateFromPassword(data, bcrypt.DefaultCost)
	return string(pw), err
}

func newHtpasswdUserCache(userFile string, syncInterval time.Duration) *htpasswdUserCache {
	stopCtx, cancel := context.WithCancel(context.Background())
	userFileObject, err := htpasswd.New(userFile, htpasswd.DefaultSystems, nil)
	if err != nil {
		panic(err)
	}
	return &htpasswdUserCache{
		userFile:       userFile,
		stopCtx:        stopCtx,
		cancel:         cancel,
		userFileObject: userFileObject,
		// Removed access or updated passwords are updated according syncInterval.
		syncInterval: syncInterval,
	}
}

// Refresh reloads users from userFile.
func (huc *htpasswdUserCache) Refresh() error {
	huc.fileMutex.RLock()
	err := huc.userFileObject.Reload(nil)
	huc.fileMutex.RUnlock()
	return err
}

func (huc *htpasswdUserCache) WatchChanges() error {
	getFileStat := func() (fs.FileInfo, error) {
		huc.fileMutex.RLock()
		stat, err := os.Stat(huc.userFile)
		huc.fileMutex.RUnlock()
		return stat, err
	}

	initialStat, err := getFileStat()
	if err != nil {
		return err
	}
	for {
		stat, err := getFileStat()
		if err != nil {
			return err
		}
		if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
			err := huc.Refresh()
			if err != nil {
				return err
			}

			// reset initial stat and watch for next modification
			initialStat, err = getFileStat()
			if err != nil {
				return err
			}
		}
		select {
		case <-time.After(huc.syncInterval):
			continue
		case <-huc.stopCtx.Done():
			return nil
		}
	}
	return nil
}

func (huc *htpasswdUserCache) Close() {
	huc.cancel()
}

func (huc *htpasswdUserCache) Match(username string, password string) bool {
	return huc.userFileObject.Match(username, password)
}

func newEtcdUserCache(cluster cluster.Cluster, etcdConfig *EtcdSpec) *etcdUserCache {
	prefix := customDataPrefix
	if etcdConfig.Prefix == "" {
		prefix += "credentials/"
	} else {
		prefix += strings.TrimPrefix(etcdConfig.Prefix, "/")
	}
	logger.Infof("credentials etcd prefix %s", prefix)
	kvs, err := cluster.GetPrefix(prefix)
	if err != nil {
		panic(err)
	}
	pwReader, err := kvsToReader(kvs, etcdConfig.UsernameKey, etcdConfig.PasswordKey)
	if err != nil {
		logger.Errorf(err.Error())
	}
	userFileObject, err := htpasswd.NewFromReader(pwReader, htpasswd.DefaultSystems, nil)
	if err != nil {
		panic(err)
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	return &etcdUserCache{
		userFileObject: userFileObject,
		cluster:        cluster,
		prefix:         prefix,
		usernameKey:    etcdConfig.UsernameKey,
		passwordKey:    etcdConfig.PasswordKey,
		cancel:         cancel,
		stopCtx:        stopCtx,
		// cluster.Syncer updates changes (removed access or updated passwords) immediately.
		// syncInterval defines data consistency check interval.
		syncInterval: 30 * time.Minute,
	}
}

func parseYamlCreds(entry string) (map[string]interface{}, error) {
	var err error
	defer func() {
		if err := recover(); err != nil {
			err = fmt.Errorf("could not marshal credentials, ensure that credentials are valid yaml")
		}
	}()
	credentials := make(map[string]interface{})
	yamltool.Unmarshal([]byte(entry), &credentials)
	return credentials, err
}

func kvsToReader(kvs map[string]string, usernameKey string, passwordKey string) (io.Reader, error) {
	reader := bytes.NewReader([]byte(""))
	pwStrSlice := make([]string, 0, len(kvs))
	for _, yaml := range kvs {
		credentials, err := parseYamlCreds(yaml)
		if err != nil {
			return reader, err
		}
		var ok bool
		username, ok := credentials[usernameKey]
		if !ok {
			return reader,
				fmt.Errorf("Parsing password updates failed. Make sure that '" +
					usernameKey + "' is a valid yaml entry.")
		}
		password, ok := credentials[passwordKey]
		if !ok {
			return reader,
				fmt.Errorf("Parsing password updates failed. Make sure that '" +
					passwordKey + "' is a valid yaml entry.")
		}
		pwStrSlice = append(pwStrSlice, username.(string)+":"+password.(string))
	}
	stringData := strings.Join(pwStrSlice, "\n")
	return strings.NewReader(stringData), nil
}

func (euc *etcdUserCache) WatchChanges() error {
	var (
		syncer *cluster.Syncer
		err    error
		ch     <-chan map[string]string
	)

	for {
		syncer, err = euc.cluster.Syncer(euc.syncInterval)
		if err != nil {
			logger.Errorf("failed to create syncer: %v", err)
		} else if ch, err = syncer.SyncPrefix(euc.prefix); err != nil {
			logger.Errorf("failed to sync prefix: %v", err)
			syncer.Close()
		} else {
			break
		}

		select {
		case <-time.After(10 * time.Second):
		case <-euc.stopCtx.Done():
			return nil
		}
	}
	defer syncer.Close()

	for {
		select {
		case <-euc.stopCtx.Done():
			return nil
		case kvs := <-ch:
			logger.Infof("basic auth credentials update")
			pwReader, err := kvsToReader(kvs, euc.usernameKey, euc.passwordKey)
			if err != nil {
				logger.Errorf(err.Error())
			}
			euc.userFileObject.ReloadFromReader(pwReader, nil)
		}
	}
	return nil
}

func (euc *etcdUserCache) Close() {
	euc.cancel()
}

func (euc *etcdUserCache) Refresh() error { return nil }

func (euc *etcdUserCache) Match(username string, password string) bool {
	return euc.userFileObject.Match(username, password)
}

// NewBasicAuthValidator creates a new Basic Auth validator
func NewBasicAuthValidator(spec *BasicAuthValidatorSpec, supervisor *supervisor.Supervisor) *BasicAuthValidator {
	var cache AuthorizedUsersCache
	if spec.Etcd != nil {
		if supervisor == nil || supervisor.Cluster() == nil {
			logger.Errorf("BasicAuth validator : failed to read data from etcd")
		} else {
			cache = newEtcdUserCache(supervisor.Cluster(), spec.Etcd)
		}
	} else if spec.UserFile != "" {
		cache = newHtpasswdUserCache(spec.UserFile, 1*time.Minute)
	} else {
		logger.Errorf("BasicAuth validator spec unvalid.")
		return nil
	}
	go cache.WatchChanges()
	bav := &BasicAuthValidator{
		spec:                 spec,
		authorizedUsersCache: cache,
	}
	return bav
}

func parseBasicAuthorizationHeader(hdr *httpheader.HTTPHeader) (string, error) {
	const prefix = "Basic "

	tokenStr := hdr.Get("Authorization")
	if !strings.HasPrefix(tokenStr, prefix) {
		return "", fmt.Errorf("unexpected authorization header: %s", tokenStr)
	}
	return strings.TrimPrefix(tokenStr, prefix), nil
}

// Validate validates the Authorization header of a http request
func (bav *BasicAuthValidator) Validate(req httpcontext.HTTPRequest) error {
	hdr := req.Header()
	base64credentials, err := parseBasicAuthorizationHeader(hdr)
	if err != nil {
		return err
	}
	credentialBytes, err := base64.StdEncoding.DecodeString(base64credentials)
	if err != nil {
		return fmt.Errorf("error occured during base64 decode: %s", err.Error())
	}
	credentials := string(credentialBytes)
	userID, password, err := parseCredentials(credentials)
	if err != nil {
		return fmt.Errorf("unauthorized")
	}

	if bav.authorizedUsersCache.Match(userID, password) {
		req.Header().Set("X-AUTH-USER", userID)
		return nil
	}
	return fmt.Errorf("unauthorized")
}

// Close closes authorizedUsersCache.
func (bav *BasicAuthValidator) Close() {
	bav.authorizedUsersCache.Close()
}
