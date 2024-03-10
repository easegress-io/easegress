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

// Package fasttime provides fast time.Now() and time.Since() and time.Format().
package fasttime

import (
	"fmt"
	"time"
)

// Since returns the elapsed time.
func Since(t time.Time) time.Duration {
	return Now().Sub(t)
}

func formatDateTime(buf []byte, t time.Time) {
	// early bounds check to guarantee safety of writes below to improve performance
	_ = buf[18]

	y, M, d := t.Date()
	buf[0] = byte(y/1000) + '0'
	buf[1] = byte(y/100%10) + '0'
	buf[2] = byte(y/10%10) + '0'
	buf[3] = byte(y%10) + '0'
	buf[4] = '-'
	buf[5] = byte(M)/10 + '0'
	buf[6] = byte(M)%10 + '0'
	buf[7] = '-'
	buf[8] = byte(d)/10 + '0'
	buf[9] = byte(d)%10 + '0'

	buf[10] = 'T'

	h, m, s := t.Clock()
	buf[11] = byte(h)/10 + '0'
	buf[12] = byte(h)%10 + '0'
	buf[13] = ':'
	buf[14] = byte(m)/10 + '0'
	buf[15] = byte(m)%10 + '0'
	buf[16] = ':'
	buf[17] = byte(s)/10 + '0'
	buf[18] = byte(s)%10 + '0'
}

var powersOf10 = [9]int64{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8}

func formatFractional(buf []byte, t time.Time, n int) int {
	// early bounds check to guarantee safety of writes below to improve performance
	_ = buf[n]

	f := t.UnixNano() % 1e9 / powersOf10[9-n]

	// The fractional part (including the dot) is omitted if its value is 0,
	// this is to align with the behavior of the standard Go library.
	if f == 0 {
		return 0
	}

	buf[0] = '.'

	d := 0
	for i := n; i > 0; i-- {
		v := f % 10
		if v != 0 && d == 0 {
			d = i
		}
		buf[i] = byte(v) + '0'
		f /= 10
	}

	return d + 1
}

func formatTimeZone(buf []byte, t time.Time) int {
	// early bounds check to guarantee safety of writes below to improve performance
	_ = buf[5]

	_, o := t.Zone()
	o /= 60
	if o == 0 {
		buf[0] = 'Z'
		return 1
	}

	if o > 0 {
		buf[0] = '+'
	} else if o < 0 {
		buf[0] = '-'
		o = -o
	}

	v := o / 60
	buf[1] = byte(v/10) + '0'
	buf[2] = byte(v%10) + '0'
	buf[3] = ':'

	v = o % 60
	buf[4] = byte(v/10) + '0'
	buf[5] = byte(v%10) + '0'

	return 6
}

// Layout is the layout to format a time value
type Layout int

const (
	// RFC3339 is the layout for RFC3339
	RFC3339 Layout = iota
	// RFC3339Milli is the layout for RFC3339 in milli-second
	RFC3339Milli
	// RFC3339Nano is the layout for RFC3339 in nano-second
	RFC3339Nano
)

// Format is equivlant with time.Format for layouts: RFC3339, RFC3339Milli and RFC3339Nano,
// but with better performance
func Format(t time.Time, layout Layout) string {
	d, m := 0, 0

	switch layout {
	case RFC3339:
	case RFC3339Milli:
		d = 4
	case RFC3339Nano:
		d = 10
	default:
		panic(fmt.Sprint("unknown layout:", layout))
	}

	buf := make([]byte, 25+d)
	formatDateTime(buf, t)
	if d > 0 {
		m = formatFractional(buf[19:], t, d-1)
	}
	n := formatTimeZone(buf[19+m:], t)
	return string(buf[:19+m+n])
}
