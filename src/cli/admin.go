package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/hexdecteam/easegateway-go-client/rest/1.0/admin/v1/pdu"
	"github.com/urfave/cli"
)

func CreatePlugin(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source string, data []byte) {
		req := new(pdu.PluginCreationRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := adminApi().CreatePlugin(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}
	for _, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		do(file, data)
	}

	return errs.Return()
}

func DeletePlugin(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) == 0 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	}

	for _, pluginName := range args {
		resp, err := adminApi().DeletePluginByName(pluginName)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			continue
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pluginName, resp.Error.Error))
			continue
		}
	}

	return errs.Return()
}

func RetrievePlugins(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(pluginName string) {
		retrieveResp, apiResp, err := adminApi().GetPluginByName(pluginName)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pluginName, apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pluginName, err))
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	doAll := func() {
		req := new(pdu.PluginsRetrieveRequest)
		retrieveResp, apiResp, err := adminApi().GetPlugins(req)
		if err != nil {
			errs.append(fmt.Errorf("%v", err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s", apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(err)
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	if len(args) == 0 {
		doAll()
	} else {
		for _, pluginName := range args {
			do(pluginName)
		}
	}

	return errs.Return()
}

func UpdatePlugin(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source string, data []byte) {
		req := new(pdu.PluginUpdateRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := adminApi().UpdatePlugin(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}
	for _, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		do(file, data)
	}

	return errs.Return()
}

func CreatePipeline(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source string, data []byte) {
		req := new(pdu.PipelineCreationRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := adminApi().CreatePipeline(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}
	for _, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		do(file, data)
	}

	return errs.Return()
}

func DeletePipeline(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	if len(args) == 0 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	}

	for _, pipelineName := range args {
		resp, err := adminApi().DeletePipelineByName(pipelineName)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, resp.Error.Error))
			continue
		}
	}

	return errs.Return()
}

func RetrievePipelines(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(pipelineName string) {
		retrieveResp, apiResp, err := adminApi().GetPipelineByName(pipelineName)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	doAll := func() {
		req := new(pdu.PipelinesRetrieveRequest)
		retrieveResp, apiResp, err := adminApi().GetPipelines(req)
		if err != nil {
			errs.append(fmt.Errorf("%v", err))
			return
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s", apiResp.Error.Error))
			return
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(err)
			return
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	if len(args) == 0 {
		doAll()
	} else {
		for _, pipelineName := range args {
			do(pipelineName)
		}
	}

	return errs.Return()
}

func UpdatePipeline(c *cli.Context) error {
	args := c.Args()

	errs := &multipleErr{}

	do := func(source string, data []byte) {
		req := new(pdu.PipelineUpdateRequest)
		err := json.Unmarshal(data, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := adminApi().UpdatePipeline(req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}
	for _, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		do(file, data)
	}

	return errs.Return()
}

func RetrievePluginTypes(c *cli.Context) error {
	errs := &multipleErr{}

	retrieveResp, apiResp, err := adminApi().GetPluginTypes()
	if err != nil {
		errs.append(err)
		return errs.Return()
	} else if apiResp.Error != nil {
		errs.append(fmt.Errorf("%s", apiResp.Error.Error))
		return errs.Return()
	}

	data, err := json.Marshal(retrieveResp)
	if err != nil {
		errs.append(err)
		return errs.Return()
	}

	// TODO: make it pretty
	fmt.Printf("%s\n", data)

	return errs.Return()
}

func RetrievePipelineTypes(c *cli.Context) error {
	errs := &multipleErr{}

	retrieveResp, apiResp, err := adminApi().GetPipelineTypes()
	if err != nil {
		errs.append(err)
		return errs.Return()
	} else if apiResp.Error != nil {
		errs.append(fmt.Errorf("%s", apiResp.Error.Error))
		return errs.Return()
	}

	data, err := json.Marshal(retrieveResp)
	if err != nil {
		errs.append(err)
		return errs.Return()
	}

	// TODO: make it pretty
	fmt.Printf("%s\n", data)

	return errs.Return()
}
