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

package nginx

import (
	"fmt"
	"testing"

	crossplane "github.com/nginxinc/nginx-go-crossplane"
	"github.com/stretchr/testify/assert"
)

func TestAddFilenameToPayload(t *testing.T) {
	tempDir := newTempTestDir(t)
	defer tempDir.Clean()

	var checkFilename func(ds crossplane.Directives, filename string)
	checkFilename = func(ds crossplane.Directives, filename string) {
		for _, d := range ds {
			assert.Equal(t, filename, d.File, "line %d, directive %v", d.Line, d)
			checkFilename(d.Block, filename)
		}
	}

	{
		nginxConf := `
		 events {}
		 http {
			 include %s;
			 server {
				 listen 80;
				 location / {
					 proxy_pass http://localhost:8888;
				 }
			 }
		 }
		 `
		proxyConf := `proxy_set_header Conf-One One;`

		proxyFile := tempDir.Create("proxy.conf", []byte(proxyConf))
		nginxFile := tempDir.Create("nginx.conf", []byte(fmt.Sprintf(nginxConf, proxyFile)))

		payload, err := crossplane.Parse(nginxFile, &crossplane.ParseOptions{})
		addFilenameToPayload(payload)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(payload.Config))
		checkFilename(payload.Config[0].Parsed, "nginx.conf")
		checkFilename(payload.Config[1].Parsed, "proxy.conf")
	}
}

func TestLoadIncludes(t *testing.T) {
	tempDir := newTempTestDir(t)
	defer tempDir.Clean()

	{
		nginxConf := `
		 events {}
		 http {
			 include %s;
			 server {
				 listen 80;
				 location / {
					 proxy_pass http://localhost:8888;
				 }
			 }
		 }
		 `
		proxyConf1 := `include %s; proxy_set_header Conf-One One;`
		proxyConf2 := `proxy_set_header Conf-Two Two;`

		proxyFile2 := tempDir.Create("proxy2.conf", []byte(proxyConf2))
		proxyFile1 := tempDir.Create("proxy1.conf", []byte(fmt.Sprintf(proxyConf1, proxyFile2)))
		nginxFile := tempDir.Create("nginx.conf", []byte(fmt.Sprintf(nginxConf, proxyFile1)))

		payload, err := crossplane.Parse(nginxFile, &crossplane.ParseOptions{})
		addFilenameToPayload(payload)
		assert.Nil(t, err)
		httpDirectives := payload.Config[0].Parsed[1].Block
		httpDirectives = loadIncludes(httpDirectives, payload)
		assert.Equal(t, 3, len(httpDirectives))

		// first directive from conf2
		d2 := httpDirectives[0]
		assert.Equal(t, "proxy_set_header", d2.Directive)
		assert.Equal(t, []string{"Conf-Two", "Two"}, d2.Args)
		assert.Equal(t, "proxy2.conf", d2.File)
		// second directive from conf1
		d1 := httpDirectives[1]
		assert.Equal(t, "proxy_set_header", d1.Directive)
		assert.Equal(t, []string{"Conf-One", "One"}, d1.Args)
		assert.Equal(t, "proxy1.conf", d1.File)
	}

	{
		// test invalid includes
		nginxConf := `
		 events {}
		 http {
			 include not-exist.conf;
			 include %s invalid-args.conf;
			 include;
			 server {
				 listen 80;
				 location / {
					 proxy_pass http://localhost:8888;
				 }
			 }
		 }
		 `
		proxyConf1 := `include %s; proxy_set_header Conf-One One;`
		proxyConf2 := `proxy_set_header Conf-Two Two;`

		proxyFile2 := tempDir.Create("proxy2.conf", []byte(proxyConf2))
		proxyFile1 := tempDir.Create("proxy1.conf", []byte(fmt.Sprintf(proxyConf1, proxyFile2)))
		nginxFile := tempDir.Create("nginx.conf", []byte(fmt.Sprintf(nginxConf, proxyFile1)))

		payload, err := crossplane.Parse(nginxFile, &crossplane.ParseOptions{})
		addFilenameToPayload(payload)
		assert.Nil(t, err)
		httpDirectives := payload.Config[0].Parsed[1].Block
		httpDirectives = loadIncludes(httpDirectives, payload)
		assert.Equal(t, 1, len(httpDirectives))
	}
}

func TestParsePayload(t *testing.T) {
	tempDir := newTempTestDir(t)
	defer tempDir.Clean()
	{
		nginxConf := `
		events {}
		http {
			upstream backend {
				server localhost:1234;
				server localhost:2345 weight=10;
			}

			server {
				listen 80;

				location / {
					proxy_set_header X-Path "prefix";
					proxy_pass http://localhost:8080;
				}

				location /apis {
					proxy_set_header X-Path "apis";
					proxy_pass http://localhost:8880;

					location /apis/v1 {
						proxy_set_header X-Path "apis/v1";
						proxy_pass http://localhost:8888;
					}
				}

				location = /user {
					proxy_pass http://localhost:8890;
				}

				location /upstream {
					proxy_pass http://backend;
				}
			}
		}
		`
		file := tempDir.Create("nginx.conf", []byte(nginxConf))
		payload, err := crossplane.Parse(file, &crossplane.ParseOptions{})
		assert.Nil(t, err)
		config, err := parsePayload(payload)
		assert.Nil(t, err)
		// printJson(config)

		opt := &Options{Prefix: "test"}
		opt.init()
		hs, pls, err := convertConfig(opt, config)
		assert.Nil(t, err)
		printYaml(hs)
		printYaml(pls)
	}
}
