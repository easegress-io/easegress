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

func TestLoadIncludes(t *testing.T) {
	tempDir := newTempTestDir(t)
	defer tempDir.Clean()

	{
		conf := `
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
		conf1 := `include %s; proxy_set_header Conf-One One;`
		conf2 := `proxy_set_header Conf-Two Two;`

		file2 := tempDir.Create(t, []byte(conf2))
		file1 := tempDir.Create(t, []byte(fmt.Sprintf(conf1, file2)))
		file := tempDir.Create(t, []byte(fmt.Sprintf(conf, file1)))

		payload, err := crossplane.Parse(file, &crossplane.ParseOptions{})
		assert.Nil(t, err)
		httpDirectives := payload.Config[0].Parsed[1].Block
		httpDirectives = loadIncludes(file, httpDirectives, payload)
		assert.Equal(t, 3, len(httpDirectives))

		// first directive from conf2
		d2 := httpDirectives[0]
		assert.Equal(t, "proxy_set_header", d2.Directive)
		assert.Equal(t, []string{"Conf-Two", "Two"}, d2.Args)
		assert.Equal(t, file2, d2.File)
		// second directive from conf1
		d1 := httpDirectives[1]
		assert.Equal(t, "proxy_set_header", d1.Directive)
		assert.Equal(t, []string{"Conf-One", "One"}, d1.Args)
		assert.Equal(t, file1, d1.File)
	}

	{
		// test invalid includes
		conf := `
		events {}
		http {
			include not-exist.conf;
			include %s invalid-args.conf;
			include;
			include %s;
			server {
				listen 80;
				location / {
					proxy_pass http://localhost:8888;
				}
			}
		}
		`
		conf1 := `include %s; proxy_set_header Conf-One One;`
		conf2 := `proxy_set_header Conf-Two Two;`

		file2 := tempDir.Create(t, []byte(conf2))
		file1 := tempDir.Create(t, []byte(fmt.Sprintf(conf1, file2)))
		file := tempDir.Create(t, []byte(fmt.Sprintf(conf, file1, file1)))

		payload, err := crossplane.Parse(file, &crossplane.ParseOptions{})
		assert.Nil(t, err)
		httpDirectives := payload.Config[0].Parsed[1].Block
		httpDirectives = loadIncludes(file, httpDirectives, payload)
		assert.Equal(t, 3, len(httpDirectives))

		// first directive from conf2
		d2 := httpDirectives[0]
		assert.Equal(t, "proxy_set_header", d2.Directive)
		assert.Equal(t, []string{"Conf-Two", "Two"}, d2.Args)
		assert.Equal(t, file2, d2.File)
		// second directive from conf1
		d1 := httpDirectives[1]
		assert.Equal(t, "proxy_set_header", d1.Directive)
		assert.Equal(t, []string{"Conf-One", "One"}, d1.Args)
		assert.Equal(t, file1, d1.File)
	}
}
