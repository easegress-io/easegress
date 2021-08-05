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

package proxy

import (
	"bytes"
	"io"
	"testing"
	"testing/iotest"
)

func TestReader(t *testing.T) {
	var b bytes.Buffer
	// Write strings to the Buffer.
	b.WriteString("ABC")

	r := bytes.NewReader(b.Bytes())

	reader1, reader2 := newMasterSlaveReader(r)

	buff := make([]byte, 10)
	n, _ := reader1.Read(buff)

	if n == 0 {
		t.Errorf("master read failed")
	}

	buff1 := make([]byte, 10)
	n, _ = reader2.Read(buff1)
	if n == 0 {
		t.Errorf("slave read failed")
	}

	if string(buff) != string(buff1) {
		t.Errorf("buff: %s, buff1: %s not the same ", string(buff), string(buff1))
	}

	reader1.Close()
}

func TestEOFReader(t *testing.T) {
	eofReader := iotest.ErrReader(io.EOF)

	reader1, reader2 := newMasterSlaveReader(eofReader)

	buff := make([]byte, 10)
	_, err := reader1.Read(buff)

	if err == nil {
		t.Errorf("master read should failed, but succ")
	}

	buff1 := make([]byte, 10)
	_, err = reader2.Read(buff1)
	if err == nil {
		t.Errorf("slave read should failed but succ ")
	}

	if string(buff) != string(buff1) {
		t.Errorf("buff: %s, buff1: %s not the same ", string(buff), string(buff1))
	}

	reader1.Close()
}
