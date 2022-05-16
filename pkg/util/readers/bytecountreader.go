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

package readers

import (
	"io"
)

// ByteCountReader wraps an io.Reader and counts the bytes read from it.
type ByteCountReader struct {
	r         io.Reader
	bytesRead int
}

// NewByteCountReader wraps an io.Reader to ByteCountReader and returns it.
func NewByteCountReader(r io.Reader) *ByteCountReader {
	return &ByteCountReader{r: r}
}

// Read implements io.Reader.
func (r *ByteCountReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.bytesRead += n
	return n, err
}

// BytesRead returns the count of bytes has been read from the underlying
// io.Reader.
func (r *ByteCountReader) BytesRead() int {
	return r.bytesRead
}

// Close implements io.Closer and closes the underlying io.Reader if
// it is an io.Closer.
func (r *ByteCountReader) Close() error {
	if c, ok := r.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
