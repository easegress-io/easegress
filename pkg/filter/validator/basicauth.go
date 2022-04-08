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
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/tg123/go-htpasswd"
	"golang.org/x/crypto/bcrypt"

	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/cluster"
	httpcontext "github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

type (
	// BasicAuthValidatorSpec defines the configuration of Basic Auth validator.
	// There are 'file' and 'etcd' modes.
	BasicAuthValidatorSpec struct {
		Mode string `yaml:"mode" jsonschema:"omitempty,enum=FILE,enum=ETCD"`
		// Required for 'FILE' mode.
		// UserFile is path to file containing encrypted user credentials in apache2-utils/htpasswd format.
		// To add user `userY`, use `sudo htpasswd /etc/apache2/.htpasswd userY`
		// Reference: https://manpages.debian.org/testing/apache2-utils/htpasswd.1.en.html#EXAMPLES
		UserFile string `yaml:"userFile" jsonschema:"omitempty"`
		// Required for 'ETCD' mode.
		// When EtcdPrefix is specified, verify user credentials from etcd. Etcd should store them:
		// key: /custom-data/{etcdPrefix}/{$key}
		// value:
		//   key: "$key"
		//   username: "$username" # optional
		//   password: "$password"
		// Username and password are used for Basic Authentication. If "username" is empty, the value of "key"
		// entry is used as username for Basic Auth.
		EtcdPrefix string `yaml:"etcdPrefix" jsonschema:"omitempty"`
	}

	// AuthorizedUsersCache provides cached lookup for authorized users.
	AuthorizedUsersCache interface {
		Match(string, string) bool
		WatchChanges()
		Close()
	}

	htpasswdUserCache struct {
		userFile       string
		userFileObject *htpasswd.File
		watcher        *fsnotify.Watcher
		syncInterval   time.Duration
		stopCtx        context.Context
		cancel         context.CancelFunc
	}

	etcdUserCache struct {
		userFileObject *htpasswd.File
		cluster        cluster.Cluster
		prefix         string
		syncInterval   time.Duration
		stopCtx        context.Context
		cancel         context.CancelFunc
	}

	// BasicAuthValidator defines the Basic Auth validator
	BasicAuthValidator struct {
		spec                 *BasicAuthValidatorSpec
		authorizedUsersCache AuthorizedUsersCache
	}

	// etcdCredentials defines the format for credentials in etcd
	etcdCredentials struct {
		Key  string `yaml:"key" jsonschema:"omitempty"`
		User string `yaml:"username" jsonschema:"omitempty"`
		Pass string `yaml:"password" jsonschema:"required"`
	}
)

const (
	customDataPrefix = "/custom-data/"
)

// Username uses username if present, otherwise key
func (cred *etcdCredentials) Username() string {
	if cred.User != "" {
		return cred.User
	}
	return cred.Key
}

// Password returns password.
func (cred *etcdCredentials) Password() string {
	return cred.Pass
}

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
	if userFile == "" {
		userFile = "/etc/apache2/.htpasswd"
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	userFileObject, err := htpasswd.New(userFile, htpasswd.DefaultSystems, nil)
	if err != nil {
		logger.Errorf(err.Error())
		userFileObject = nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Errorf(err.Error())
		watcher = nil
	}
	return &htpasswdUserCache{
		userFile:       userFile,
		stopCtx:        stopCtx,
		cancel:         cancel,
		watcher:        watcher,
		userFileObject: userFileObject,
		// Removed access or updated passwords are updated according syncInterval.
		syncInterval: syncInterval,
	}
}

func (huc *htpasswdUserCache) WatchChanges() {
	if huc.userFileObject == nil || huc.watcher == nil {
		return
	}
	go func() {
		for {
			select {
			case _, ok := <-huc.watcher.Events:
				if !ok {
					return
				}
				err := huc.userFileObject.Reload(nil)
				if err != nil {
					logger.Errorf(err.Error())
				}
			case err, ok := <-huc.watcher.Errors:
				if !ok {
					return
				}
				logger.Errorf(err.Error())
			}
		}
	}()
	err := huc.watcher.Add(huc.userFile)
	if err != nil {
		logger.Errorf(err.Error())
	}
	return
}

func (huc *htpasswdUserCache) Close() {
	if huc.watcher != nil {
		huc.watcher.Close()
	}
}

func (huc *htpasswdUserCache) Match(username string, password string) bool {
	return huc.userFileObject.Match(username, password)
}

func newEtcdUserCache(cluster cluster.Cluster, etcdPrefix string) *etcdUserCache {
	prefix := customDataPrefix
	if etcdPrefix == "" {
		prefix += "credentials/"
	} else {
		prefix = customDataPrefix + strings.TrimPrefix(etcdPrefix, "/")
	}
	logger.Infof("credentials etcd prefix %s", prefix)
	kvs, err := cluster.GetPrefix(prefix)
	if err != nil {
		logger.Errorf(err.Error())
		return &etcdUserCache{}
	}
	pwReader := kvsToReader(kvs)
	userFileObject, err := htpasswd.NewFromReader(pwReader, htpasswd.DefaultSystems, nil)
	if err != nil {
		logger.Errorf(err.Error())
		return &etcdUserCache{}
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	return &etcdUserCache{
		userFileObject: userFileObject,
		cluster:        cluster,
		prefix:         prefix,
		cancel:         cancel,
		stopCtx:        stopCtx,
		// cluster.Syncer updates changes (removed access or updated passwords) immediately.
		// syncInterval defines data consistency check interval.
		syncInterval: 30 * time.Minute,
	}
}

func kvsToReader(kvs map[string]string) io.Reader {
	pwStrSlice := make([]string, 0, len(kvs))
	for _, item := range kvs {
		creds := &etcdCredentials{}
		err := yaml.Unmarshal([]byte(item), creds)
		if err != nil {
			logger.Errorf(err.Error())
			continue
		}
		if creds.Username() == "" || creds.Password() == "" {
			logger.Errorf(
				"Parsing credential updates failed. " +
					"Make sure that credentials contains 'key' or 'password' entry for password and 'password' entries.",
			)
			continue
		}
		pwStrSlice = append(pwStrSlice, creds.Username()+":"+creds.Password())
	}
	if len(pwStrSlice) == 0 {
		// no credentials found, let's return empty reader
		return bytes.NewReader([]byte(""))
	}
	stringData := strings.Join(pwStrSlice, "\n")
	return strings.NewReader(stringData)
}

func (euc *etcdUserCache) WatchChanges() {
	if euc.prefix == "" {
		logger.Errorf("missing etcd prefix, skip watching changes")
		return
	}
	var (
		syncer cluster.Syncer
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
			return
		}
	}
	// start listening in background
	go func() {
		defer syncer.Close()

		for {
			select {
			case <-euc.stopCtx.Done():
				return
			case kvs := <-ch:
				logger.Infof("basic auth credentials update")
				pwReader := kvsToReader(kvs)
				euc.userFileObject.ReloadFromReader(pwReader, nil)
			}
		}
	}()
	return
}

func (euc *etcdUserCache) Close() {
	if euc.prefix == "" {
		return
	}
	euc.cancel()
}

func (euc *etcdUserCache) Match(username string, password string) bool {
	if euc.prefix == "" {
		return false
	}
	return euc.userFileObject.Match(username, password)
}

// NewBasicAuthValidator creates a new Basic Auth validator
func NewBasicAuthValidator(spec *BasicAuthValidatorSpec, supervisor *supervisor.Supervisor) *BasicAuthValidator {
	var cache AuthorizedUsersCache
	switch spec.Mode {
	case "ETCD":
		if supervisor == nil || supervisor.Cluster() == nil {
			logger.Errorf("BasicAuth validator : failed to read data from etcd")
			return nil
		}
		cache = newEtcdUserCache(supervisor.Cluster(), spec.EtcdPrefix)
	case "FILE":
		cache = newHtpasswdUserCache(spec.UserFile, 1*time.Minute)
	default:
		logger.Errorf("BasicAuth validator spec unvalid.")
		return nil
	}
	cache.WatchChanges()
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
