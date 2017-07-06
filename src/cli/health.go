package cli

import (
	"encoding/json"
	"fmt"

	"github.com/urfave/cli"
)

func Check(c *cli.Context) error {
	resp, err := healthApi().Check()
	if err != nil {
		return fmt.Errorf("%v\n", err)
	} else if resp.Error != nil {
		return fmt.Errorf("%s\n", resp.Error.Error)
	}

	fmt.Println("Service is reachable.")

	return nil
}

func Info(c *cli.Context) error {
	info, resp, err := healthApi().GetInfo()
	if err != nil {
		return fmt.Errorf("%v\n", err)
	} else if resp.Error != nil {
		return fmt.Errorf("%s\n", resp.Error.Error)
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	// TODO: make it pretty
	fmt.Printf("%s\n", data)
	return nil
}
