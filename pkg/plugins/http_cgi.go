package plugins

import (
	"encoding/base64"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/option"
)

var (
	TrailingPort = regexp.MustCompile(`:([0-9]+)$`)
)

func UpperCaseAndUnderscore(s string) string {
	return strings.Map(upperCaseAndUnderscore, s)
}

func upperCaseAndUnderscore(r rune) rune {
	switch {
	case r >= 'a' && r <= 'z':
		return r - ('a' - 'A')
	case r == '-':
		return '_'
	}
	return r
}

// https://tools.ietf.org/html/rfc3875#section-4.1
func GenerateCGIEnv(c HTTPCtx) (map[string]string, []string) {
	if c == nil {
		return nil, nil
	}

	header := c.RequestHeader()
	var (
		// 4.1.1 - 4.1.17
		authType         string
		contentLength    string
		contentType      string
		gatewayInterface string
		pathInfo         string
		pathTranslated   string
		queryString      string
		remoteAddr       string
		remoteHost       string
		remoteIdent      string
		remoteUser       string
		requestMethod    string
		scriptName       string
		serverName       string
		serverPort       string
		serverProtocol   string
		serverSoftware   string
	)

	// 4.1.1
	authValues := strings.Fields(header.Get("Authorization"))
	if len(authValues) != 0 {
		switch authValues[0] {
		case "Basic", "Digest", "Bearer":
			authType = authValues[0]
		}
	}

	// 4.1.2
	if header.ContentLength() > 0 {
		contentLength = fmt.Sprintf("%d", header.ContentLength())
	}

	// 4.1.3
	contentType = header.Get("Content-Type")

	// 4.1.4
	gatewayInterface = "CGI/1.1"

	// 4.1.5
	// FIXME: Corresponding to the definition of RFC,
	// Currently the design chose req.URL.Path as value of PATH_INFO,
	// but maybe there is more precise options.
	pathInfo = header.Path()

	// 4.1.6
	pathTranslated = filepath.Join(option.CGIDir, pathInfo)

	// 4.1.7
	queryString = header.QueryString()

	// 4.1.8 & 4.1.9
	remoteIP, _, err := net.SplitHostPort(c.RemoteAddr())
	if err != nil {
		remoteAddr, remoteHost = c.RemoteAddr(), c.RemoteAddr()
	} else {
		remoteAddr, remoteHost = remoteIP, remoteIP
	}

	// 4.1.10
	remoteIdent = ""

	// 4.1.11
	switch authType {
	case "Basic", "Digest":
		if len(authValues) > 1 {
			decoded, err := base64.StdEncoding.DecodeString(authValues[1])
			if err == nil {
				userPassword := strings.Split(string(decoded), ":")
				if len(userPassword) != 0 {
					remoteUser = userPassword[0]
				}
			}
		}
	}

	// 4.1.12
	requestMethod = header.Method()

	// 4.1.13
	// FIXEME: Currently got no idea about it.
	scriptName = ""

	// 4.1.14
	serverName = header.Host()

	// 4.1.15
	serverPort = "80"
	if matches := TrailingPort.FindStringSubmatch(serverName); len(matches) != 0 {
		serverPort = matches[1]
	}

	// 4.1.16
	serverProtocol = header.Proto()
	// 4.1.17
	serverSoftware = "ease-gateway"

	env := map[string]string{
		"AUTH_TYPE":         authType,         // 4.1.1
		"CONTENT_LENGTH":    contentLength,    // 4.1.2
		"CONTENT_TYPE":      contentType,      // 4.1.3
		"GATEWAY_INTERFACE": gatewayInterface, // 4.1.4
		"PATH_INFO":         pathInfo,         // 4.1.5
		"PATH_TRANSLATED":   pathTranslated,   // 4.1.6
		"QUERY_STRING":      queryString,      // 4.1.7
		"REMOTE_ADDR":       remoteAddr,       // 4.1.8
		"REMOTE_HOST":       remoteHost,       // 4.1.9
		"REMOTE_IDENT":      remoteIdent,      // 4.1.10
		"REMOTE_USER":       remoteUser,       // 4.1.11
		"REQUEST_METHOD":    requestMethod,    // 4.1.12
		"SCRIPT_NAME":       scriptName,       // 4.1.13
		"SERVER_NAME":       serverName,       // 4.1.14
		"SERVER_PORT":       serverPort,       // 4.1.15
		"SERVER_PROTOCOL":   serverProtocol,   // 4.1.16
		"SERVER_SOFTWARE":   serverSoftware,   // 4.1.17
	}

	// Names of Protocol-Specific Meta-Variables
	var names []string

	// 4.1.18
	header.VisitAll(func(key, value string) {
		k := UpperCaseAndUnderscore(key)
		v := value
		// NOTICE: Gateway seems not have this problem. because we
		// not going to cover system environment.
		// if k == "PROXY" {
		//	// https://github.com/golang/go/issues/16405
		//	continue
		// }
		joinStr := ", "
		if k == "COOKIE" {
			joinStr = "; "
		}
		name := "HTTP_" + k
		names = append(names, name)
		if env[name] == "" {
			env[name] = v
		} else {
			env[name] = env[name] + joinStr + v
		}
	})

	return env, names
}
