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

// Package filterwriter provides a filter writer.
package filterwriter

import (
	"io"
)

// FilterFunc is a filter function for the Filter Writer,
// returns true to write 'p' to the underlying writer,
// return false to discard.
type FilterFunc func(p []byte) bool

type filterWriter struct {
	filter FilterFunc
	w      io.Writer
}

// New creates a Filter Writer
func New(w io.Writer, filter FilterFunc) io.Writer {
	return &filterWriter{
		filter: filter,
		w:      w,
	}
}

// Write implements io.Writer
func (fw *filterWriter) Write(p []byte) (int, error) {
	if fw.filter(p) {
		return fw.w.Write(p)
	}
	return len(p), nil
}
