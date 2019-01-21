package command

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func APICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api list",
		Short: "List available APIs",
		Run:   apiListFunc,
	}

	return cmd
}

func apiListFunc(cmd *cobra.Command, args []string) {
	req, err := http.NewRequest(http.MethodGet, makeURL(apiURL), nil)
	if err != nil {
		ExitWithError(err)
	}

	_, body, err := handleRequest(req)
	if err != nil {
		ExitWithErrorf("list apis failed: %v", err)
	}

	// TODO: make it pretty?
	fmt.Printf("%s", body)
}
