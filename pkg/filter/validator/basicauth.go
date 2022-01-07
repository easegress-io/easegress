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
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/tg123/go-htpasswd"

	"github.com/megaease/easegress/pkg/cluster"
	httpcontext "github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

type (
	// BasicAuthValidatorSpec defines the configuration of Basic Auth validator.
	// Only one of UserFile or UseEtcd should be defined.
	BasicAuthValidatorSpec struct {
		// UserFile is path to file containing encrypted user credentials in apache2-utils/htpasswd format.
		// To add user `userY`, use `sudo htpasswd /etc/apache2/.htpasswd userY`
		// Reference: https://manpages.debian.org/testing/apache2-utils/htpasswd.1.en.html#EXAMPLES
		UserFile string `yaml:"userFile" jsonschema:"omitempty"`
		// When UseEtcd is true, verify user credentials from etcd. Etcd stores them:
		// key: /credentials/{user id}
		// value: {encrypted password}
		UseEtcd bool `yaml:"useEtcd" jsonschema:"omitempty"`
	}

	// AuthorizedUsersCache provides cached lookup for authorized users.
	AuthorizedUsersCache interface {
		Match(string, string) bool
		WatchChanges() error
		Refresh() error
		Close()
	}

	htpasswdUserCache struct {
		cache          *lru.Cache
		userFile       string
		userFileObject *htpasswd.File
		fileMutex      sync.RWMutex
		syncInterval   time.Duration
		stopCtx        context.Context
		cancel         context.CancelFunc
	}

	etcdUserCache struct {
		cache        *lru.Cache
		cluster      cluster.Cluster
		syncInterval time.Duration
		stopCtx      context.Context
		cancel       context.CancelFunc
	}

	// BasicAuthValidator defines the Basic Auth validator
	BasicAuthValidator struct {
		spec                 *BasicAuthValidatorSpec
		authorizedUsersCache AuthorizedUsersCache
	}
)

const credsPrefix = "/credentials/"

func parseCredentials(creds string) (string, string, error) {
	parts := strings.Split(creds, ":")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("bad format")
	}
	return parts[0], parts[1], nil
}

func sha256Sum(data []byte) string {
	sha256Bytes := sha256.Sum256(data)
	return hex.EncodeToString(sha256Bytes[:])
}

func newHtpasswdUserCache(userFile string, syncInterval time.Duration) *htpasswdUserCache {
	cache, err := lru.New(256)
	if err != nil {
		panic(err)
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	userFileObject, err := htpasswd.New(userFile, htpasswd.DefaultSystems, nil)
	if err != nil {
		panic(err)
	}
	return &htpasswdUserCache{
		cache:          cache,
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

func newEtcdUserCache(cluster cluster.Cluster) *etcdUserCache {
	cache, err := lru.New(256)
	if err != nil {
		panic(err)
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	return &etcdUserCache{
		cache:   cache,
		cluster: cluster,
		cancel:  cancel,
		stopCtx: stopCtx,
		// cluster.Syncer updates changes (removed access or updated passwords) immediately.
		// syncInterval defines data consistency check interval.
		syncInterval: 30 * time.Minute,
	}
}

func (euc *etcdUserCache) GetUser(targetUserID string) (string, bool) {
	if val, ok := euc.cache.Get(targetUserID); ok {
		return val.(string), true
	}

	password, err := euc.cluster.Get(credsPrefix + targetUserID)
	if err != nil {
		logger.Errorf(err.Error())
		return "", false
	}
	if password == nil {
		logger.Errorf("unauthorized")
		return "", false
	}

	euc.cache.Add(targetUserID, *password)
	return *password, true
}

// updateCache updates the intersection of kvs map and cache and removes other keys from cache.
func updateCache(kvs map[string]string, cache *lru.Cache) {
	intersection := make(map[string]string)
	for key, password := range kvs {
		userID := strings.TrimPrefix(key, credsPrefix)
		if oldPassword, ok := cache.Peek(userID); ok {
			intersection[userID] = password
			if password != oldPassword {
				cache.Add(userID, password) // update password
			}
		}
	}
	// delete cache items that were not in kvs
	for _, cacheKey := range cache.Keys() {
		if _, exists := intersection[cacheKey.(string)]; !exists {
			cache.Remove(cacheKey)
		}
	}
}

func (euc *etcdUserCache) WatchChanges() error {
	var (
		syncer *cluster.Syncer
		err    error
		ch     <-chan map[string]string
	)

	for {
		syncer, err = euc.cluster.Syncer(20 * time.Minute)
		if err != nil {
			logger.Errorf("failed to create syncer: %v", err)
		} else if ch, err = syncer.SyncPrefix(credsPrefix); err != nil {
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
			updateCache(kvs, euc.cache)
		}
	}
	return nil
}

func (euc *etcdUserCache) Close() {
	euc.cancel()
}

func (euc *etcdUserCache) Refresh() error { return nil }

func (euc *etcdUserCache) Match(targetUserID string, targetPassword string) bool {
	var password string
	if val, ok := euc.cache.Get(targetUserID); ok {
		password = val.(string)
	} else {
		result, err := euc.cluster.Get(credsPrefix + targetUserID)
		if err != nil {
			logger.Errorf(err.Error())
			return false
		}
		if result == nil {
			logger.Errorf("unauthorized")
			return false
		}
		password = *result
		euc.cache.Add(targetUserID, password)
	}
	return password == targetPassword
}

// NewBasicAuthValidator creates a new Basic Auth validator
func NewBasicAuthValidator(spec *BasicAuthValidatorSpec, supervisor *supervisor.Supervisor) *BasicAuthValidator {
	var cache AuthorizedUsersCache
	if spec.UseEtcd {
		if supervisor == nil || supervisor.Cluster() == nil {
			logger.Errorf("BasicAuth validator : failed to read data from etcd")
		} else {
			cache = newEtcdUserCache(supervisor.Cluster())
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
		return nil
	}
	return fmt.Errorf("unauthorized")
}

// Close closes authorizedUsersCache.
func (bav *BasicAuthValidator) Close() {
	bav.authorizedUsersCache.Close()
}
