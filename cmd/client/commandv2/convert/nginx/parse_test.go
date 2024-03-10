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

package nginx

import (
	"bytes"
	"fmt"
	"html/template"
	"testing"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
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

				server_name www.example.com *.example.com;

				location /apis {
					proxy_set_header X-Path "apis";
					proxy_pass http://localhost:8880;

					location /apis/v1 {
						proxy_set_header X-Path "apis/v1";
						proxy_pass http://localhost:8888;
					}
				}

				location /upstream {
					gzip on;
					gzip_min_length 1000;
					proxy_pass http://backend;
				}

				location /websocket {
					proxy_http_version 1.1;
					proxy_set_header Upgrade $http_upgrade;
					proxy_set_header Connection $connection_upgrade;
					proxy_pass http://localhost:9090;
				}
			}

			server {
				listen 127.0.0.1:443 ssl;
				ssl_client_certificate {{ .CaFile }};
				ssl_certificate {{ .CertFile }};
				ssl_certificate_key {{ .KeyFile }};

				location = /user {
					proxy_pass http://localhost:9999;
				}

				location ^~ /user/admin {
					proxy_pass http://localhost:9991;
				}

				location ~* /user/.* {
					proxy_pass http://localhost:9992;
				}

				location ~ /user/.* {
					proxy_pass http://localhost:9993;
				}
			}
		}
		`
		tmplValue := map[string]string{
			"CaFile":   tempDir.Create("ca.crt", []byte("ca")),
			"CertFile": tempDir.Create("cert.crt", []byte("cert")),
			"KeyFile":  tempDir.Create("key.crt", []byte("key")),
		}

		tmpl, err := template.New("nginx").Parse(nginxConf)
		assert.Nil(t, err)
		var buffer bytes.Buffer
		tmpl.Execute(&buffer, tmplValue)

		file := tempDir.Create("nginx.conf", buffer.Bytes())
		payload, err := crossplane.Parse(file, &crossplane.ParseOptions{})
		assert.Nil(t, err)
		config, err := parsePayload(payload)
		assert.Nil(t, err)

		proxyInfo := `
servers:
    - port: 80
      rules:
        - hosts:
            - value: www.example.com
              isRegexp: false
            - value: '*.example.com'
              isRegexp: false
          paths:
            - path: /apis
              type: prefix
              backend:
                servers:
                    - server: http://localhost:8880
                      weight: 1
                setHeaders:
                    X-Path: apis
            - path: /apis/v1
              type: prefix
              backend:
                servers:
                    - server: http://localhost:8888
                      weight: 1
                setHeaders:
                    X-Path: apis/v1
            - path: /upstream
              type: prefix
              backend:
                servers:
                    - server: http://localhost:1234
                      weight: 1
                    - server: http://localhost:2345
                      weight: 10
                gzipMinLength: 1000
            - path: /websocket
              type: prefix
              backend:
                servers:
                    - server: http://localhost:9090
                      weight: 1
                setHeaders:
                    Connection: $connection_upgrade
                    Upgrade: $http_upgrade
    - port: 443
      address: 127.0.0.1
      https: true
      caCert: Y2E=
      certs:
        {{ .CertFile }}: Y2VydA==
      keys:
        {{ .CertFile }}: a2V5
      rules:
        - paths:
            - path: /user
              type: exact
              backend:
                servers:
                    - server: http://localhost:9999
                      weight: 1
            - path: /user/admin
              type: prefix
              backend:
                servers:
                    - server: http://localhost:9991
                      weight: 1
            - path: /user/.*
              type: caseInsensitiveRegexp
              backend:
                servers:
                    - server: http://localhost:9992
                      weight: 1
            - path: /user/.*
              type: regexp
              backend:
                servers:
                    - server: http://localhost:9993
                      weight: 1
`
		tmp, err := template.New("proxyInfo").Parse(proxyInfo)
		assert.Nil(t, err)
		var proxyBuffer bytes.Buffer
		tmp.Execute(&proxyBuffer, tmplValue)
		expected := &Config{}
		err = codectool.UnmarshalYAML(proxyBuffer.Bytes(), expected)
		assert.Nil(t, err)
		assert.Equal(t, expected, config)
	}
}

func TestUpdateServer(t *testing.T) {
	tempDir := newTempTestDir(t)
	defer tempDir.Clean()
	{
		nginxConf := `
		events {}
		http {
			server {
				listen 80;

				location /apis {
					proxy_pass http://localhost:8880;
				}
			}

			server {
				listen 80;

				location /user {
					proxy_pass http://localhost:9999;
				}
			}
		}
		`
		file := tempDir.Create("nginx.conf", []byte(nginxConf))
		payload, err := crossplane.Parse(file, &crossplane.ParseOptions{})
		assert.Nil(t, err)
		config, err := parsePayload(payload)
		assert.Nil(t, err)

		proxyInfo := `
servers:
    - port: 80
      rules:
        - paths:
            - path: /apis
              type: prefix
              backend:
                servers:
                    - server: http://localhost:8880
                      weight: 1
        - paths:
            - path: /user
              type: prefix
              backend:
                servers:
                    - server: http://localhost:9999
                      weight: 1
`
		expected := &Config{}
		err = codectool.UnmarshalYAML([]byte(proxyInfo), expected)
		assert.Nil(t, err)
		assert.Equal(t, expected, config)
	}
	{
		servers := []*Server{
			{
				ServerBase: ServerBase{
					Port:  80,
					Certs: map[string]string{},
					Keys:  map[string]string{},
				},
			},
		}
		server := &Server{
			ServerBase: ServerBase{
				Port:   80,
				HTTPS:  true,
				CaCert: "ca",
				Certs:  map[string]string{"cert": "cert"},
				Keys:   map[string]string{"cert": "key"},
			},
		}
		servers, err := updateServers(servers, server)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(servers))
		got := servers[0]
		assert.True(t, got.HTTPS)
		assert.Equal(t, "ca", got.CaCert)
		assert.Equal(t, map[string]string{"cert": "cert"}, got.Certs)
		assert.Equal(t, map[string]string{"cert": "key"}, got.Keys)
	}
}
