package command

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

func appendError(err, err1 error) error {
	if err != nil {
		return fmt.Errorf("%s\n%s", err, err1)
	}

	return err1
}

// ExitWithError exits with self-defined message not the one of cobra(such as usage).
func ExitWithError(err error) {
	if err != nil {
		color.New(color.FgRed).Fprint(os.Stderr, "Error: ")
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// ExitWithErrorf wraps ExitWithError with format.
func ExitWithErrorf(format string, a ...interface{}) {
	ExitWithError(fmt.Errorf(format, a...))
}
