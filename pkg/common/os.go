package common

import (
	"fmt"
	"os"
)

func Exit(code int, msg string) {
	if code != 0 {
		if msg != "" {
			fmt.Fprintf(os.Stderr, "%s\n", msg)
		}
		os.Exit(code)
	}

	if msg != "" {
		fmt.Fprintf(os.Stdout, "%v\n", msg)
	}
	os.Exit(0)
}
