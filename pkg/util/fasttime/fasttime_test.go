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

package fasttime

import (
	"fmt"
	"testing"
	"time"
)

func TestNow(t *testing.T) {
	now := Now()
	nano := NowUnixNano()
	duration := Since(now)
	fmt.Printf("now: %v, unix %v since: %v\n", now, nano, duration)
}

func BenchmarkStdNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		time.Now()
	}
}

func BenchmarkNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Now()
	}
}

func BenchmarkNowUnixNano(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NowUnixNano()
	}
}

var fmtCases = []time.Time{
	time.Date(2021, 8, 1, 0, 0, 0, 0, time.UTC),
	time.Date(2021, 12, 15, 10, 20, 30, 123456000, time.UTC),
	time.Date(2021, 12, 15, 10, 20, 30, 123456001, time.UTC),

	time.Date(2021, 8, 1, 0, 0, 0, 0, time.Local),
	time.Date(2021, 12, 15, 10, 20, 30, 123456000, time.Local),
	time.Date(2021, 12, 15, 10, 20, 30, 123456001, time.Local),

	time.Date(2021, 8, 1, 0, 0, 0, 0, time.FixedZone("UTC-8", -8*60*60)),
	time.Date(2021, 12, 15, 10, 20, 30, 123456000, time.FixedZone("UTC-8", -8*60*60)),
	time.Date(2021, 12, 15, 10, 20, 30, 123456001, time.FixedZone("UTC-8", -8*60*60)),
}

func TestRFC3339(t *testing.T) {
	for _, now := range fmtCases {
		a := Format(now, RFC3339)
		b := now.Format(time.RFC3339)
		if a != b {
			t.Errorf("RFC3339 is not correct, should be %s, but get %s", b, a)
		}
	}
}

func TestRFC3339Milli(t *testing.T) {
	for _, now := range fmtCases {
		a := Format(now, RFC3339Milli)
		b := now.Format("2006-01-02T15:04:05.999Z07:00")
		if a != b {
			t.Errorf("RFC3339Milli is not correct, should be %s, but get %s", b, a)
		}
	}
}

func TestRFC3339Nano(t *testing.T) {
	for _, now := range fmtCases {
		a := Format(now, RFC3339Nano)
		b := now.Format(time.RFC3339Nano)
		if a != b {
			t.Errorf("RFC3339Nano is not correct, should be %s, but get %s", b, a)
		}
	}
}

func BenchmarkStdRFC3339(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		now.Format(time.RFC3339)
	}
}

func BenchmarkRFC3339(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		Format(now, RFC3339)
	}
}

func BenchmarkStdRFC3339Milli(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		now.Format("2006-01-02T15:04:05.999Z07:00")
	}
}

func BenchmarkRFC3339Milli(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		Format(now, RFC3339Milli)
	}
}

func BenchmarkStdRFC3339Nano(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		now.Format(time.RFC3339Nano)
	}
}

func BenchmarkRFC3339Nano(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		Format(now, RFC3339Nano)
	}
}
