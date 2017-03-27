package common

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	trailingPort = regexp.MustCompile(`:([0-9]+)$`)
)

// https://tools.ietf.org/html/rfc3875#section-4.1
func GenerateCGIEnv(req *http.Request) map[string]string {
	if req == nil {
		return nil
	}

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
	authValues := strings.Fields(req.Header.Get("Authorization"))
	if len(authValues) != 0 {
		switch authValues[0] {
		case "Basic", "Digest", "Bearer":
			authType = authValues[0]
		}
	}

	// 4.1.2
	if req.ContentLength > 0 {
		contentLength = fmt.Sprintf("%d", req.ContentLength)
	}

	// 4.1.3
	contentType = req.Header.Get("Content-Type")

	// 4.1.4
	gatewayInterface = "CGI/1.1"

	// 4.1.5
	// FIXME: Corresponding to the definition of RFC,
	// Currently the design chose req.URL.Path as value of PATH_INFO,
	// but maybe there is more precise options.
	pathInfo = req.URL.Path

	// 4.1.6
	pathTranslated = filepath.Join(CGI_HOME_DIR, pathInfo)

	// 4.1.7
	queryString = req.URL.RawQuery

	// 4.1.8 & 4.1.9
	remoteIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		remoteAddr, remoteHost = req.RemoteAddr, req.RemoteAddr
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
	requestMethod = req.Method

	// 4.1.13
	// FIXEME: Currently got no idea about it.
	scriptName = ""

	// 4.1.14
	serverName = req.Host

	// 4.1.15
	serverPort = "80"
	if matches := trailingPort.FindStringSubmatch(req.Host); len(matches) != 0 {
		serverPort = matches[1]
	}

	// 4.1.16
	serverProtocol = req.Proto

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

	// 4.1.18
	for k, v := range req.Header {
		k = UpperCaseAndUnderscore(k)
		// NOTICE: Gateway seems not have this problem. because we
		// not going to cover system environment.
		// if k == "PROXY" {
		// 	// https://github.com/golang/go/issues/16405
		// 	continue
		// }
		joinStr := ", "
		if k == "COOKIE" {
			joinStr = "; "
		}
		env["HTTP_"+k] = strings.Join(v, joinStr)
	}

	return env
}

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
