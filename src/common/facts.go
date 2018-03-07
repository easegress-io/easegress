package common

import (
	"os"
	"path/filepath"
)

const (
	DEFAULT_PAGE  = 1
	DEFAULT_LIMIT_PER_PAGE = 5
)

var (
	BIN_IMAGE_DIR, _    = filepath.Abs(filepath.Dir(os.Args[0]))
	WORKING_HOME_DIR, _ = filepath.Abs(filepath.Join(BIN_IMAGE_DIR, ".."))
	LOG_HOME_DIR        = filepath.Join(WORKING_HOME_DIR, "logs")
	PLUGIN_HOME_DIR     = filepath.Join(WORKING_HOME_DIR, "plugins")
	INVENTORY_HOME_DIR  = filepath.Join(WORKING_HOME_DIR, "inventory")
	CONFIG_HOME_DIR     = filepath.Join(INVENTORY_HOME_DIR, "config")
	CERT_HOME_DIR       = filepath.Join(INVENTORY_HOME_DIR, "cert")
	CGI_HOME_DIR        = filepath.Join(INVENTORY_HOME_DIR, "cgi")
)
