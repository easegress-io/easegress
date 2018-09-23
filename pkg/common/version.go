package common

import (
	"fmt"
	"strings"
)

const (
	APIVersion = "v1"
)

func PrefixAPIVersion(path string) string {
	var prefix string
	if strings.HasPrefix(path, "/") {
		prefix = fmt.Sprintf("/%s", APIVersion)
	} else {
		prefix = fmt.Sprintf("/%s/", APIVersion)
	}
	return prefix + path
}
