package cli

import (
	"fmt"

	"github.com/urfave/cli"
)

func CheckHealth(c *cli.Context) error {
	resp, err := healthApi().Check()
	if err != nil {
		return fmt.Errorf("%s\n", err)
	} else if resp.Error != nil {
		return fmt.Errorf("%s\n", resp.Error.Error)
	}

	return nil
}
