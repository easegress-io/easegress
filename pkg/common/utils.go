package common

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const TOKEN_ESCAPE_CHAR string = `\`

func StrInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// GraphiteSplit slices s into all substrings separated by sep
// returns a slice of the substrings without length prefix separated by lensep.
// The routine does its best to split without error returned.
func GraphiteSplit(s string, lensep string, sep string) []string {
	a := make([]string, 0)

	i := 0
	n := len(s)
	for i < n {
		j := strings.Index(s[i:], lensep)
		if j == -1 {
			return a
		}
		j += i

		length, err := strconv.ParseUint(s[i:j], 10, 16)
		if err != nil {
			return a
		}

		i = j + len(lensep)
		j = i + int(length)
		if j > n {
			return a
		}

		a = append(a, s[i:j])

		i = j
		if i < n && s[i:i+len(sep)] != sep {
			return a
		}

		i += len(sep)
	}
	return a
}

type TokenVisitor func(pos int, token string) (care bool, replacement string)

func ScanTokens(str string, removeEscapeChar bool, visitor TokenVisitor) (string, error) {
	if visitor == nil {
		visitor = func(_ int, _ string) (bool, string) {
			return false, ""
		}
	}

	in := false
	ret := bytes.NewBuffer(nil)
	v := []byte(str)

	escaped := func(pos int) bool {
		if pos == 0 {
			return false
		}

		return string(v[pos-1]) == TOKEN_ESCAPE_CHAR
	}

	escaper := func(s string) string {
		return strings.Replace(
			strings.Replace(s, TOKEN_ESCAPE_CHAR+`{`, "{", -1),
			TOKEN_ESCAPE_CHAR+`}`, "}", -1)
	}

	token := bytes.NewBuffer(nil)
	for i, c := range v {
		if c == '{' && !escaped(i) {
			if in {
				return str, fmt.Errorf("invalid pattern string")
			}

			in = true
		} else if c == '}' && !escaped(i) {
			if !in {
				return str, fmt.Errorf("invalid pattern string")
			}

			if len(strings.TrimSpace(token.String())) == 0 {
				return str, fmt.Errorf("empty token")
			}

			pos := i - token.Len() - 1
			token = bytes.NewBufferString(escaper(token.String()))

			care, replacement := visitor(pos, token.String())
			if care {
				ret.WriteString(replacement)
				c = 0
			} else {
				ret.WriteString("{")
				ret.WriteString(token.String())
			}

			token = bytes.NewBuffer(nil)
			in = false
		} else if in {
			token.WriteByte(c)
		}

		if !in && c != 0 {
			ret.WriteByte(c)
		}
	}

	if in {
		return str, fmt.Errorf("invalid pattern string")
	}

	retStr := ret.String()
	if removeEscapeChar {
		retStr = escaper(retStr)
	}

	return retStr, nil
}

func PanicToErr(f func(), err *error) (failed bool) {
	defer func() {
		x := recover()
		if x == nil {
			failed = false
			return
		}

		failed = true

		if err == nil {
			return
		}

		switch e := x.(type) {
		case error:
			*err = e
		case string:
			*err = fmt.Errorf(e)
		default:
			*err = fmt.Errorf("%v", x)
		}

		return
	}()

	f()

	return
}

var (
	TRUE_STRINGS  = []string{"1", "t", "true", "on", "y", "yes"}
	FALSE_STRINGS = []string{"0", "f", "false", "off", "n", "no"}
)

func BoolFromStr(s string, def bool) bool {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	if StrInSlice(s, TRUE_STRINGS) {
		return true
	} else if StrInSlice(s, FALSE_STRINGS) {
		return false
	} else {
		return def
	}
}

func RemoveRepeatedByte(s string, needRemoveByte byte) string {
	if len(s) < 2 {
		return s
	}

	out := NewLazybuf(s)
	repeatingByte := s[0]
	out.Append(repeatingByte)
	for _, c := range []byte(s[1:]) {
		if c != repeatingByte || c != needRemoveByte {
			out.Append(c)
		}
		repeatingByte = c
	}
	return out.String()
}

// Via: https://stackoverflow.com/a/466242/1705845
//      https://graphics.stanford.edu/~seander/bithacks.html#RoundUpPowerOf2
func NextNumberPowerOf2(v uint64) uint64 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}

// safe characters for friendly url, rfc3986 section 2.3
var URL_FRIENDLY_CHARACTERS_REGEX = regexp.MustCompile(`^[A-Za-z0-9\-_\.~]+$`)
