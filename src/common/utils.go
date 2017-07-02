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

func ScanTokens(str string) ([]string, error) {
	in := false
	ret := make([]string, 0)

	token := ""
	for _, c := range str {
		if c == '{' {
			if in {
				return nil, fmt.Errorf("invalid pattern string")
			} else {
				token = ""
				in = true
			}
		} else if c == '}' {
			if in {
				if len(strings.TrimSpace(token)) == 0 {
					return nil, fmt.Errorf("empty token")
				}
				ret = append(ret, token)
				in = false
			} else {
				return nil, fmt.Errorf("invalid pattern string")
			}
		} else if in {
			token += string(c)
		}
	}

	if in {
		return nil, fmt.Errorf("invalid pattern string")
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
