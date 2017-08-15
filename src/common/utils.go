package common

import (
	"fmt"
	"strconv"
	"strings"
)

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
	var ret string
	v := []byte(str)

	escaped := func(pos int) bool {
		if pos == 0 {
			return false
		}

		return v[pos-1] == '\\'
	}

	token := ""
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

			if len(strings.TrimSpace(token)) == 0 {
				return str, fmt.Errorf("empty token")
			}

			pos := i - len(token) - 1
			token = strings.Replace(strings.Replace(token, `\{`, "{", -1), `\}`, "}", -1)

			care, replacement := visitor(pos, token)
			if care {
				ret += replacement
				c = 0
			} else {
				ret += fmt.Sprintf("{%s", token)
			}

			token = ""
			in = false
		} else if in {
			token += string(c)
		}

		if !in && c != 0 {
			ret += string(c)
		}
	}

	if in {
		return str, fmt.Errorf("invalid pattern string")
	}

	if removeEscapeChar {
		ret = strings.Replace(strings.Replace(ret, `\{`, "{", -1), `\}`, "}", -1)
	}

	return ret, nil
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
