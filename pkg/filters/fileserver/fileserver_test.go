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

package fileserver

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsFileHidden(t *testing.T) {
	assert := assert.New(t)
	testCases := []struct {
		hidden []string
		path   string
		expect bool
	}{
		{
			hidden: nil,
			path:   "/path",
			expect: false,
		},
		{
			hidden: []string{"*.log"},
			path:   "/var/logs/app.log",
			expect: true,
		},
		{
			hidden: []string{"*.log"},
			path:   "/var/logs/app.log.gz",
			expect: false,
		},
		{
			hidden: []string{"node_modules"},
			path:   "/home/user/project/node_modules/react/index.js",
			expect: true,
		},
		{
			hidden: []string{"secret"},
			path:   "/my_secrets/file",
			expect: false,
		},
		{
			hidden: []string{"/private"},
			path:   "/private.txt",
			expect: false,
		},
		{
			hidden: []string{"/private"},
			path:   "/private/data/keys/001.key",
			expect: true,
		},
		{
			hidden: []string{"/config.json"},
			path:   "/config.json",
			expect: true,
		},
		{
			hidden: []string{"/home/*/config/*.conf"},
			path:   "/home/alice/config/app.conf",
			expect: true,
		},
		{
			hidden: []string{"/home/*/config/*.conf"},
			path:   "/home/alice/settings/app.conf",
			expect: false,
		},
		{
			hidden: []string{"/*.conf"},
			path:   "/config/app.conf",
			expect: false,
		},
		{
			hidden: []string{"/build/"},
			path:   "/build/index.html",
			expect: true,
		},
		{
			hidden: []string{"/build/"},
			path:   "/build",
			expect: false,
		},
		{
			hidden: []string{"/data", "cache"},
			path:   "/tmp/cache/images/pic.jpg",
			expect: true,
		},
		{
			hidden: []string{"temp", "/logs"},
			path:   "/logs/error.log",
			expect: true,
		},
		{
			hidden: []string{"/"},
			path:   "/",
			expect: true,
		},
		{
			hidden: []string{".*"},
			path:   "/home/user/.bashrc",
			expect: true,
		},
		{
			hidden: []string{".env"},
			path:   "/var/www/project/.env",
			expect: true,
		},
		{
			hidden: []string{".*"},
			path:   "/home/user/profile",
			expect: false,
		},
		{
			hidden: []string{"/data"},
			path:   "/data",
			expect: true,
		},
		{
			hidden: []string{"/build/"},
			path:   "/build",
			expect: false,
		},
		{
			hidden: []string{"/var/log/app?.log"},
			path:   "/var/log/app1.log",
			expect: true,
		},
		{
			hidden: []string{"/var/log/app?.log"},
			path:   "/var/log/app10.log",
			expect: false,
		},
		{
			hidden: []string{"tmp"},
			path:   "/var//tmp/session",
			expect: true,
		},
		{
			hidden: []string{"*"},
			path:   "/anything/here",
			expect: true,
		},
		{
			hidden: []string{"*"},
			path:   "/",
			expect: true,
		},
		{
			hidden: []string{"/a/b/c"},
			path:   "/a/b",
			expect: false,
		},
	}

	for _, tc := range testCases {
		if runtime.GOOS == "windows" {
			tc.path = toWindowsPath(tc.path)
			for i := range tc.hidden {
				tc.hidden[i] = toWindowsPath(tc.hidden[i])
			}
		}
		fs := &FileServer{
			spec: &Spec{
				Hidden: tc.hidden,
			},
		}
		fs.Init()
		actual := fs.isFileHidden(tc.path)
		assert.Equal(tc.expect, actual, tc)
		fs.Close()
	}
}

func toWindowsPath(path string) string {
	if strings.HasPrefix(path, "/") {
		path, _ = filepath.Abs(path)
	}
	return filepath.FromSlash(path)
}
