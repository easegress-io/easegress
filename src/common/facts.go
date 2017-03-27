package common

import (
	"flag"
	"os"
	"path/filepath"
)

var (
	SCRIPT_BIN_DIR, _   = filepath.Abs(filepath.Dir(os.Args[0]))
	WORKING_HOME_DIR, _ = filepath.Abs(filepath.Join(SCRIPT_BIN_DIR, ".."))
	LOG_HOME_DIR        = filepath.Join(WORKING_HOME_DIR, "logs")
	INVENTORY_HOME_DIR  = filepath.Join(WORKING_HOME_DIR, "inventory")
	CONFIG_HOME_DIR     = filepath.Join(INVENTORY_HOME_DIR, "config")
	CERT_HOME_DIR       = filepath.Join(INVENTORY_HOME_DIR, "cert")
	CGI_HOME_DIR        = filepath.Join(INVENTORY_HOME_DIR, "cgi")

	Host  string
	Stage string
)

func init() {
	host := flag.String("host", "localhost", "specify host to corresponding cert files")
	stage := flag.String("stage", "debug", "sepcify runtime stage(debug,test,prod)")

	flag.Parse()

	Host = *host
	Stage = *stage
}
