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

package sampler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSampler(t *testing.T) {
	s := NewDurationSampler()
	p := s.Percentiles()
	assert.Equal(t, 7, len(p))
	assert.Equal(t, 0.0, p[0])
	assert.Equal(t, 0.0, p[1])
	assert.Equal(t, 0.0, p[2])

	s.Update(time.Millisecond)
	p = s.Percentiles()
	assert.Equal(t, 7, len(p))
	assert.Equal(t, 1.0, p[0])
	assert.Equal(t, 1.0, p[1])
	assert.Equal(t, 1.0, p[6])

	s.Update(1000 * time.Second)
	p = s.Percentiles()
	assert.Equal(t, 7, len(p))
	assert.Equal(t, 1.0, p[0])
	assert.Equal(t, 1.0, p[1])
	assert.Equal(t, 257000.0, p[2])

	s.Reset()
	p = s.Percentiles()
	assert.Equal(t, 7, len(p))
	assert.Equal(t, 0.0, p[0])
	assert.Equal(t, 0.0, p[1])
	assert.Equal(t, 0.0, p[2])
}
