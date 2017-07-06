package cli

import (
	"encoding/json"
	"fmt"

	"github.com/urfave/cli"
)

func Check(c *cli.Context) error {
	errs := &multipleErr{}

	apiResp, err := healthApi().Check()
	if err != nil {
		errs.append(err)
		return errs.Return()
	} else if apiResp.Error != nil {
		errs.append(fmt.Errorf("%s", apiResp.Error.Error))
		return errs.Return()
	}

	fmt.Println("Service is reachable.")

	return errs.Return()
}

func Info(c *cli.Context) error {
	errs := &multipleErr{}

	info, apiResp, err := healthApi().GetInfo()
	if err != nil {
		errs.append(err)
		return errs.Return()
	} else if apiResp.Error != nil {
		errs.append(fmt.Errorf("%s", apiResp.Error.Error))
		return errs.Return()
	}

	data, err := json.Marshal(info)
	if err != nil {
		errs.append(err)
		return errs.Return()
	}

	// TODO: make it pretty
	fmt.Printf("%s\n", data)

	return errs.Return()
}
