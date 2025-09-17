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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func newContext(t *testing.T, req *http.Request, rw http.ResponseWriter) *context.Context {
	ctx := context.New(nil)
	httpreq, err := httpprot.NewRequest(req)
	assert.Nil(t, err)
	ctx.SetRequest("default", httpreq)
	ctx.SetData("HTTP_RESPONSE_WRITER", rw)
	ctx.UseNamespace("default")
	return ctx
}

func TestMain(m *testing.M) {
	logger.InitNop()
	os.Exit(m.Run())
}

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
		fmt.Println("hidden:", tc.hidden, "path:", tc.path)
		fmt.Println("fs:", fs.spec, "fs:", fs)
		input := &filePath{path: tc.path}
		fs.setFileHidden(input)
		assert.Equal(tc.expect, input.isHidden, tc)
		fs.Close()
	}
}

func TestFileNeedCache(t *testing.T) {
	assert := assert.New(t)
	fs := &FileServer{
		spec: &Spec{
			Cache: CacheSpec{
				CacheFileExtensionFilters: make([]FileCacheExtensionFilter, 0),
			},
		},
	}
	fs.Init()
	fs.spec.Cache.CacheFileExtensionFilters = []FileCacheExtensionFilter{
		{Extension: []string{"*.html", "*.css"}},
	}

	testCases := []struct {
		path   string
		expect bool
	}{
		{
			path:   "/var/www/index.html",
			expect: true,
		},
		{
			path:   "/usr/app.js",
			expect: false,
		},
	}

	for _, tc := range testCases {
		if runtime.GOOS == "windows" {
			tc.path = toWindowsPath(tc.path)
			fs.spec.Cache.CacheFileExtensionFilters = []FileCacheExtensionFilter{
				{Extension: []string{toWindowsPath("*.html"), toWindowsPath("*.css")}},
			}
		}
		input := &filePath{path: tc.path}
		fs.setFileNeedCache(input)
		assert.Equal(tc.expect, input.needCache, tc)
	}
}

func TestFileServer(t *testing.T) {
	assert := assert.New(t)
	tempDir, err := os.MkdirTemp("", "go-fileserver-test-*")
	assert.Nil(err)
	defer os.RemoveAll(tempDir)

	filesToCreate := map[string]string{
		"index.html": `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Test Page</title>
    <link rel="stylesheet" href="/assets/style.css">
</head>
<body>
    <h1>test</h1>
    <p>This is a html file.</p>
</body>
</html>`,
		"hello.txt": "some text",
		"assets/style.css": `body {
    font-family: sans-serif;
    background-color: #f0f0f0;
    color: #333;
}`,
		"assets/script.js": `console.log("JavaScript!");`,
	}

	for filename, content := range filesToCreate {
		filePath := filepath.Join(tempDir, filename)

		err = os.MkdirAll(filepath.Dir(filePath), 0755)
		assert.Nil(err)

		err = os.WriteFile(filePath, []byte(content), 0644)
		assert.Nil(err)
	}

	spec := &Spec{
		Root: tempDir,
	}
	f := FileServer{
		spec: spec,
	}
	f.Init()
	defer f.Close()

	testCases := []struct {
		uri string
		key string
	}{
		{
			uri: "/",
			key: "index.html",
		},
		{
			uri: "/hello.txt",
			key: "hello.txt",
		},
		{
			uri: "/assets/style.css",
			key: "assets/style.css",
		},
		{
			uri: "/assets/script.js",
			key: "assets/script.js",
		},
	}

	for i, tc := range testCases {
		msg := fmt.Sprintf("case %d: %v", i, tc)
		req, err := http.NewRequest("GET", "http://example.com"+tc.uri, nil)
		assert.Nil(err)
		rw := httptest.NewRecorder()
		ctx := newContext(t, req, rw)
		res := f.Handle(ctx)
		assert.Equal("", res, msg)
		body := rw.Body.String()
		expectedBody, ok := filesToCreate[tc.key]
		assert.True(ok, msg)
		assert.Equal(expectedBody, body, msg)
		assert.Equal(200, rw.Code, msg)
	}
}

func toWindowsPath(path string) string {
	if strings.HasPrefix(path, "/") {
		path, _ = filepath.Abs(path)
	}
	newPath := filepath.FromSlash(path)
	if strings.HasSuffix(path, "/") {
		newPath += string(os.PathSeparator)
	}
	return newPath
}
