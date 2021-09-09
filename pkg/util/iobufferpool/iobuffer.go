/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package iobufferpool

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	AutoExpand      = -1
	MinRead         = 1 << 9
	MaxRead         = 1 << 17
	ResetOffMark    = -1
	DefaultSize     = 1 << 4
	MaxBufferLength = 1 << 20
	MaxThreshold    = 1 << 22
)

var nullByte []byte

var (
	EOF                  = errors.New("EOF")
	ErrTooLarge          = errors.New("io buffer: too large")
	ErrNegativeCount     = errors.New("io buffer: negative count")
	ErrInvalidWriteCount = errors.New("io buffer: invalid write count")
	ConnReadTimeout      = 15 * time.Second
)

type pipe struct {
	IoBuffer
	mu sync.Mutex
	c  sync.Cond

	err error
}

func (p *pipe) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.IoBuffer == nil {
		return 0
	}
	return p.IoBuffer.Len()
}

// Read waits until data is available and copies bytes
// from the buffer into p.
func (p *pipe) Read(d []byte) (n int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.c.L == nil {
		p.c.L = &p.mu
	}
	for {
		if p.IoBuffer != nil && p.IoBuffer.Len() > 0 {
			return p.IoBuffer.Read(d)
		}
		if p.err != nil {
			return 0, p.err
		}
		p.c.Wait()
	}
}

var errClosedPipeWrite = errors.New("write on closed buffer")

// Write copies bytes from p into the buffer and wakes a reader.
// It is an error to write more data than the buffer can hold.
func (p *pipe) Write(d []byte) (n int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.c.L == nil {
		p.c.L = &p.mu
	}
	defer p.c.Signal()
	if p.err != nil {
		return 0, errClosedPipeWrite
	}
	return len(d), p.IoBuffer.Append(d)
}

// CloseWithError causes the next Read (waking up a current blocked
// Read if needed) to return the provided err after all data has been
// read.
//
// The error must be non-nil.
func (p *pipe) CloseWithError(err error) {
	if err == nil {
		err = io.EOF
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.c.L == nil {
		p.c.L = &p.mu
	}
	p.err = err
	defer p.c.Signal()
}

func NewPipeBuffer(capacity int) IoBuffer {
	return &pipe{
		IoBuffer: newIoBuffer(capacity),
	}
}

// ioBuffer is an implementation of IoBuffer
type ioBuffer struct {
	buf     []byte // contents: buf[off : len(buf)]
	off     int    // read from &buf[off], write to &buf[len(buf)]
	offMark int
	count   int32
	eof     bool

	b *[]byte
}

func newIoBuffer(capacity int) IoBuffer {
	buffer := &ioBuffer{
		offMark: ResetOffMark,
		count:   1,
	}
	if capacity <= 0 {
		capacity = DefaultSize
	}
	buffer.b = GetBytes(capacity)
	buffer.buf = (*buffer.b)[:0]
	return buffer
}

func NewIoBufferString(s string) IoBuffer {
	if s == "" {
		return newIoBuffer(0)
	}
	return &ioBuffer{
		buf:     []byte(s),
		offMark: ResetOffMark,
		count:   1,
	}
}

func NewIoBufferBytes(bytes []byte) IoBuffer {
	if bytes == nil {
		return NewIoBuffer(0)
	}
	return &ioBuffer{
		buf:     bytes,
		offMark: ResetOffMark,
		count:   1,
	}
}

func NewIoBufferEOF() IoBuffer {
	buf := newIoBuffer(0)
	buf.SetEOF(true)
	return buf
}

func (b *ioBuffer) Read(p []byte) (n int, err error) {
	if b.off >= len(b.buf) {
		b.Reset()

		if len(p) == 0 {
			return
		}

		return 0, io.EOF
	}

	n = copy(p, b.buf[b.off:])
	b.off += n

	return
}

func (b *ioBuffer) Grow(n int) error {
	_, ok := b.tryGrowByReslice(n)

	if !ok {
		b.grow(n)
	}

	return nil
}

func (b *ioBuffer) ReadOnce(r io.Reader) (n int64, err error) {
	var m int

	if b.off > 0 && b.off >= len(b.buf) {
		b.Reset()
	}

	if b.off >= (cap(b.buf) - len(b.buf)) {
		b.copy(0)
	}

	// free max buffers avoid memleak
	if b.off == len(b.buf) && cap(b.buf) > MaxBufferLength {
		b.Free()
		b.Alloc(MaxRead)
	}

	l := cap(b.buf) - len(b.buf)

	m, err = r.Read(b.buf[len(b.buf):cap(b.buf)])

	b.buf = b.buf[0 : len(b.buf)+m]
	n = int64(m)

	// Not enough space anywhere, we need to allocate.
	if l == m {
		b.copy(AutoExpand)
	}

	return n, err
}

func (b *ioBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	if b.off >= len(b.buf) {
		b.Reset()
	}

	for {
		if free := cap(b.buf) - len(b.buf); free < MinRead {
			// not enough space at end
			if b.off+free < MinRead {
				// not enough space using beginning of buffer;
				// double buffer capacity
				b.copy(MinRead)
			} else {
				b.copy(0)
			}
		}

		m, e := r.Read(b.buf[len(b.buf):cap(b.buf)])

		b.buf = b.buf[0 : len(b.buf)+m]
		n += int64(m)

		if e == io.EOF {
			break
		}

		if m == 0 {
			break
		}

		if e != nil {
			return n, e
		}
	}

	return
}

func (b *ioBuffer) Write(p []byte) (n int, err error) {
	m, ok := b.tryGrowByReslice(len(p))

	if !ok {
		m = b.grow(len(p))
	}

	return copy(b.buf[m:], p), nil
}

func (b *ioBuffer) WriteString(s string) (n int, err error) {
	m, ok := b.tryGrowByReslice(len(s))

	if !ok {
		m = b.grow(len(s))
	}

	return copy(b.buf[m:], s), nil
}

func (b *ioBuffer) tryGrowByReslice(n int) (int, bool) {
	if l := len(b.buf); l+n <= cap(b.buf) {
		b.buf = b.buf[:l+n]

		return l, true
	}

	return 0, false
}

func (b *ioBuffer) grow(n int) int {
	m := b.Len()

	// If buffer is empty, reset to recover space.
	if m == 0 && b.off != 0 {
		b.Reset()
	}

	// Try to grow by means of a reslice.
	if i, ok := b.tryGrowByReslice(n); ok {
		return i
	}

	if m+n <= cap(b.buf)/2 {
		// We can slide things down instead of allocating a new
		// slice. We only need m+n <= cap(b.buf) to slide, but
		// we instead let capacity get twice as large so we
		// don't spend all our time copying.
		b.copy(0)
	} else {
		// Not enough space anywhere, we need to allocate.
		b.copy(n)
	}

	// Restore b.off and len(b.buf).
	b.off = 0
	b.buf = b.buf[:m+n]

	return m
}

func (b *ioBuffer) WriteTo(w io.Writer) (n int64, err error) {
	for b.off < len(b.buf) {
		nBytes := b.Len()
		m, e := w.Write(b.buf[b.off:])

		if m > nBytes {
			panic(ErrInvalidWriteCount)
		}

		b.off += m
		n += int64(m)

		if e != nil {
			return n, e
		}

		if m == 0 || m == nBytes {
			return n, nil
		}
	}

	return
}

func (b *ioBuffer) WriteByte(p byte) error {
	m, ok := b.tryGrowByReslice(1)

	if !ok {
		m = b.grow(1)
	}

	b.buf[m] = p
	return nil
}

func (b *ioBuffer) WriteUint16(p uint16) error {
	m, ok := b.tryGrowByReslice(2)

	if !ok {
		m = b.grow(2)
	}

	binary.BigEndian.PutUint16(b.buf[m:], p)
	return nil
}

func (b *ioBuffer) WriteUint32(p uint32) error {
	m, ok := b.tryGrowByReslice(4)

	if !ok {
		m = b.grow(4)
	}

	binary.BigEndian.PutUint32(b.buf[m:], p)
	return nil
}

func (b *ioBuffer) WriteUint64(p uint64) error {
	m, ok := b.tryGrowByReslice(8)

	if !ok {
		m = b.grow(8)
	}

	binary.BigEndian.PutUint64(b.buf[m:], p)
	return nil
}

func (b *ioBuffer) Append(data []byte) error {
	if b.off >= len(b.buf) {
		b.Reset()
	}

	dataLen := len(data)

	if free := cap(b.buf) - len(b.buf); free < dataLen {
		// not enough space at end
		if b.off+free < dataLen {
			// not enough space using beginning of buffer;
			// double buffer capacity
			b.copy(dataLen)
		} else {
			b.copy(0)
		}
	}

	m := copy(b.buf[len(b.buf):len(b.buf)+dataLen], data)
	b.buf = b.buf[0 : len(b.buf)+m]

	return nil
}

func (b *ioBuffer) AppendByte(data byte) error {
	return b.Append([]byte{data})
}

func (b *ioBuffer) Peek(n int) []byte {
	if len(b.buf)-b.off < n {
		return nil
	}

	return b.buf[b.off : b.off+n]
}

func (b *ioBuffer) Mark() {
	b.offMark = b.off
}

func (b *ioBuffer) Restore() {
	if b.offMark != ResetOffMark {
		b.off = b.offMark
		b.offMark = ResetOffMark
	}
}

func (b *ioBuffer) Bytes() []byte {
	return b.buf[b.off:]
}

func (b *ioBuffer) Cut(offset int) IoBuffer {
	if b.off+offset > len(b.buf) {
		return nil
	}

	buf := make([]byte, offset)

	copy(buf, b.buf[b.off:b.off+offset])
	b.off += offset
	b.offMark = ResetOffMark

	return &ioBuffer{
		buf: buf,
		off: 0,
	}
}

func (b *ioBuffer) Drain(offset int) {
	if b.off+offset > len(b.buf) {
		return
	}

	b.off += offset
	b.offMark = ResetOffMark
}

func (b *ioBuffer) String() string {
	return string(b.buf[b.off:])
}

func (b *ioBuffer) Len() int {
	return len(b.buf) - b.off
}

func (b *ioBuffer) Cap() int {
	return cap(b.buf)
}

func (b *ioBuffer) Reset() {
	b.buf = b.buf[:0]
	b.off = 0
	b.offMark = ResetOffMark
	b.eof = false
}

func (b *ioBuffer) available() int {
	return len(b.buf) - b.off
}

func (b *ioBuffer) Clone() IoBuffer {
	buf := GetIoBuffer(b.Len())
	buf.Write(b.Bytes())

	buf.SetEOF(b.EOF())

	return buf
}

func (b *ioBuffer) Free() {
	b.Reset()
	b.giveSlice()
}

func (b *ioBuffer) Alloc(size int) {
	if b.buf != nil {
		b.Free()
	}
	if size <= 0 {
		size = DefaultSize
	}
	b.b = b.makeSlice(size)
	b.buf = *b.b
	b.buf = b.buf[:0]
}

func (b *ioBuffer) Count(count int32) int32 {
	return atomic.AddInt32(&b.count, count)
}

func (b *ioBuffer) EOF() bool {
	return b.eof
}

func (b *ioBuffer) SetEOF(eof bool) {
	b.eof = eof
}

//The expand parameter means the following:
//A, if expand > 0, cap(newbuf) is calculated according to cap(oldbuf) and expand.
//B, if expand == AutoExpand, cap(newbuf) is calculated only according to cap(oldbuf).
//C, if expand == 0, only copy, buf not be expanded.
func (b *ioBuffer) copy(expand int) {
	var newBuf []byte
	var bufp *[]byte

	if expand > 0 || expand == AutoExpand {
		cap := cap(b.buf)
		// when buf cap greater than MaxThreshold, start Slow Grow.
		if cap < 2*MinRead {
			cap = 2 * MinRead
		} else if cap < MaxThreshold {
			cap = 2 * cap
		} else {
			cap = cap + cap/4
		}

		if expand == AutoExpand {
			expand = 0
		}

		bufp = b.makeSlice(cap + expand)
		newBuf = *bufp
		copy(newBuf, b.buf[b.off:])
		PutBytes(b.b)
		b.b = bufp
	} else {
		newBuf = b.buf
		copy(newBuf, b.buf[b.off:])
	}
	b.buf = newBuf[:len(b.buf)-b.off]
	b.off = 0
}

func (b *ioBuffer) makeSlice(n int) *[]byte {
	return GetBytes(n)
}

func (b *ioBuffer) giveSlice() {
	if b.b != nil {
		PutBytes(b.b)
		b.b = nil
		b.buf = nullByte
	}
}

func (b *ioBuffer) CloseWithError(err error) {
}
