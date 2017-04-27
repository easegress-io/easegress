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

	Host       string
	CertFile   string
	KeyFile    string
	Stage      string
	ConfigHome string
	LogHome    string
)

func init() {
	host := flag.String("host", "localhost", "specify listen host")
	certFile := flag.String("certfile", "", "specify cert file, downgrade HTTPS(10443) to HTTP(10080) if it is set empty or inexistent file")
	keyFile := flag.String("keyfile", "", "specify key file, downgrade HTTPS(10443) to HTTP(10080) if it is set empty or inexistent file")
	stage := flag.String("stage", "debug", "sepcify runtime stage (debug, test, prod)")
	configHome := flag.String("config", CONFIG_HOME_DIR, "sepcify config home path")
	logHome := flag.String("log", LOG_HOME_DIR, "specify log home path")

	flag.Parse()

	Host = *host
	CertFile = *certFile
	KeyFile = *keyFile
	Stage = *stage
	ConfigHome = *configHome
	LogHome = *logHome
}
