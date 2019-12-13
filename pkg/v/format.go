package v

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"time"
)

var (
	formatsFuncs = map[string]FormatFunc{
		"urlname":          urlName,
		"httpmethod":       httpMethod,
		"httpmethod-array": httpMethodArray,
		"httpcode":         httpCode,
		"timerfc3339":      timerfc3339,
		"duration":         duration,
		"ipcidr":           ipcidr,
		"ipcidr-array":     ipcidrArray,
		"hostport":         hostport,
		"regexp":           _regexp,
		"base64":           _base64,
	}

	urlCharsRegexp = regexp.MustCompile(`^[A-Za-z0-9\-_\.~]{1,253}$`)
)

func getFormatFunc(format string) (FormatFunc, bool) {
	switch format {
	case "date-time", "email", "hostname", "ipv4", "ipv6", "uri":
		return standardFormat, true

	case "":
		// NOTICE: Empty format does nothing like standard format.
		return standardFormat, true
	}

	if fn, exists := formatsFuncs[format]; exists {
		return fn, true
	}

	return nil, false
}

func standardFormat(v interface{}) error {
	// NOTICE: Its errors will be reported by standard json schema.
	return nil
}

func urlName(v interface{}) error {
	if urlCharsRegexp.MatchString(v.(string)) {
		return nil
	}

	return fmt.Errorf("invalid name format")
}

func httpMethod(v interface{}) error {
	switch v.(string) {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace:
		return nil
	default:
		return fmt.Errorf("invalid http method")
	}
}

func httpMethodArray(v interface{}) error {
	for _, method := range v.([]string) {
		err := httpMethod(method)
		if err != nil {
			return err
		}
	}

	return nil
}

func httpCode(v interface{}) error {
	code := v.(int)
	// Referece: https://tools.ietf.org/html/rfc7231#section-6
	if code < 100 || code >= 600 {
		return fmt.Errorf("invalid http code")
	}
	return nil
}

func timerfc3339(v interface{}) error {
	s := v.(string)
	_, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("invalid RFC3339 time: %v", err)
	}
	return nil
}

func duration(v interface{}) error {
	s := v.(string)
	_, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration: %v", err)
	}
	return nil
}

func ipcidr(v interface{}) error {
	s := v.(string)
	ip := net.ParseIP(s)
	if ip != nil {
		return nil
	}

	_, _, err := net.ParseCIDR(s)
	if err != nil {
		return fmt.Errorf("invalid ip or cidr")
	}

	return nil
}

func ipcidrArray(v interface{}) error {
	for _, ic := range v.([]string) {
		err := ipcidr(ic)
		if err != nil {
			return err
		}
	}

	return nil
}

func hostport(v interface{}) error {
	s := v.(string)
	_, _, err := net.SplitHostPort(s)
	if err != nil {
		return fmt.Errorf("invalid hostport: %v", err)
	}
	return nil
}

func _regexp(v interface{}) error {
	s := v.(string)
	_, err := regexp.Compile(s)
	if err != nil {
		return fmt.Errorf("invalid regular expression: %v", err)
	}

	return nil
}

func _base64(v interface{}) error {
	s := v.(string)
	_, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("invalid base64: %v", err)
	}

	return nil
}
