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
	"bytes"
	"io"
	"testing"
)

func TestReaderAt(t *testing.T) {
	r := bytes.NewReader([]byte("abcdefghijklmnopqrstuvwxyz"))

	ra := NewReaderAt(r)

	r1 := NewReaderAtReader(ra, 0)

	buf := make([]byte, 10)
	n, err := r1.Read(buf)
	if n != 10 || err != nil {
		t.Errorf("expect 10 bytes and no error, but got: %d, %v", n, err)
	}

	n, err = r1.Read(buf)
	if n != 10 || err != nil {
		t.Errorf("expect 10 bytes and no error, but got: %d, %v", n, err)
	}

	n, err = r1.Read(buf)
	if n != 6 {
		t.Errorf("expect 6 bytes, but got: %d", n)
	}

	n, err = r1.Read(buf)
	if n != 0 || err != io.EOF {
		t.Errorf("expect 0 bytes and EOF, but got: %d, %v", n, err)
	}

	r1 = NewReaderAtReader(ra, 7)
	n, err = r1.Read(buf)
	if n != 10 || err != nil {
		t.Errorf("expect 10 bytes and no error, but got: %d, %v", n, err)
	}

	n, err = r1.Read(buf)
	if n != 9 {
		t.Errorf("expect 6 bytes, but got: %d", n)
	}

	n, err = r1.Read(buf)
	if n != 0 || err != io.EOF {
		t.Errorf("expect 0 bytes and EOF, but got: %d, %v", n, err)
	}

	ra.Close()
}
