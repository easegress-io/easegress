package common

// NOTICE: Copy from path/path.go in standard library.

// A Lazybuf is a lazily constructed path buffer.
// It supports append, reading previously appended bytes,
// and retrieving the final string. It does not allocate a buffer
// to hold the output until that output diverges from s.
type Lazybuf struct {
	s   string
	buf []byte
	w   int
}

func NewLazybuf(s string) *Lazybuf {
	return &Lazybuf{
		s: s,
	}
}

func (b *Lazybuf) Index(i int) byte {
	if b.buf != nil {
		return b.buf[i]
	}
	return b.s[i]
}

func (b *Lazybuf) Append(c byte) {
	if b.buf == nil {
		if b.w < len(b.s) && b.s[b.w] == c {
			b.w++
			return
		}
		b.buf = make([]byte, len(b.s))
		copy(b.buf, b.s[:b.w])
	}
	b.buf[b.w] = c
	b.w++
}

func (b *Lazybuf) String() string {
	if b.buf == nil {
		return b.s[:b.w]
	}
	return string(b.buf[:b.w])
}
