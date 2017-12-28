package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway-go-client/rest/1.0/cluster/statistics/v1/pdu"
	"github.com/urfave/cli"

	"common"
)

func ClusterRetrievePluginIndicatorNames(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	}

	pipelineName := args[0]
	pluginNames := args[1:]

	var expiredTime time.Duration
	for i, pluginName := range pluginNames {
		req := new(pdu.StatisticsClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())

		if timeout <= expiredTime {
			errs.append(fmt.Errorf(
				"timeout: skip to handle [%s]", strings.Join(pluginNames[i:], ", ")))
			break
		}
		timeout -= expiredTime

		startTime := common.Now()
		retrieveResp, apiResp, err := clusterStatApi().GetPluginIndicatorNames(
			group, pipelineName, pluginName, req)
		expiredTime = common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, pluginName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, pluginName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, pluginName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func ClusterGetPluginIndicatorValue(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	} else if len(args) < 3 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]
	pluginName := args[1]
	indicatorNames := args[2:]

	var expiredTime time.Duration
	for i, indicatorName := range indicatorNames {
		req := new(pdu.StatisticsClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())

		if timeout <= expiredTime {
			errs.append(fmt.Errorf(
				"timeout: skip to handle [%s]", strings.Join(indicatorNames[i:], ", ")))
			break
		}
		timeout -= expiredTime

		startTime := common.Now()
		value, apiResp, err := clusterStatApi().GetPluginIndicatorValue(
			group, pipelineName, pluginName, indicatorName, req)
		expiredTime = common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %v",
				pipelineName, pluginName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(
				fmt.Errorf("%s-%s-%s: %s",
					pipelineName, pluginName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(value)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %v",
				pipelineName, pluginName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func ClusterGetPluginIndicatorDesc(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("plugin name requied"))
		return errs.Return()
	} else if len(args) < 3 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]
	pluginName := args[1]
	indicatorNames := args[2:]

	var expiredTime time.Duration
	for i, indicatorName := range indicatorNames {
		req := new(pdu.StatisticsClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())

		if timeout <= expiredTime {
			errs.append(fmt.Errorf(
				"timeout: skip to handle [%s]", strings.Join(indicatorNames[i:], ", ")))
			break
		}
		timeout -= expiredTime

		startTime := common.Now()
		desc, apiResp, err := clusterStatApi().GetPluginIndicatorDesc(
			group, pipelineName, pluginName, indicatorName, req)
		expiredTime = common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %v",
				pipelineName, pluginName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %s",
				pipelineName, pluginName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(desc)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s-%s: %v",
				pipelineName, pluginName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func ClusterRetrievePipelineIndicatorNames(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	}

	pipelineNames := args

	var expiredTime time.Duration
	for i, pipelineName := range pipelineNames {
		req := new(pdu.StatisticsClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())

		if timeout <= expiredTime {
			errs.append(fmt.Errorf(
				"timeout: skip to handle [%s]", strings.Join(pipelineNames[i:], ", ")))
			break
		}
		timeout -= expiredTime

		startTime := common.Now()
		retrieveResp, apiResp, err := clusterStatApi().GetPipelineIndicatorNames(group, pipelineName, req)
		expiredTime = common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func ClusterGetPipelineIndicatorsValue(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]

	req := new(pdu.ClusterPipelineIndicatorsValueRequest)
	req.TimeoutSec = uint16(timeout.Seconds())
	req.IndicatorNames = args[1:]

	retrieveResp, apiResp, err := clusterStatApi().GetPipelineIndicatorsValue(
		group, pipelineName, req)
	if err != nil {
		errs.append(fmt.Errorf("%s: %v", pipelineName, err))
	} else if apiResp.Error != nil {
		errs.append(fmt.Errorf("%s: %s", pipelineName, apiResp.Error.Error))
	} else {
		data, err := json.Marshal(retrieveResp.Values)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
		} else {
			// TODO: make it pretty
			fmt.Printf("%s\n", data)
		}
	}

	return errs.Return()
}

func ClusterGetPipelineIndicatorDesc(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]
	indicatorNames := args[1:]

	var expiredTime time.Duration
	for i, indicatorName := range indicatorNames {
		req := new(pdu.StatisticsClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())

		if timeout <= expiredTime {
			errs.append(fmt.Errorf(
				"timeout: skip to handle [%s]", strings.Join(indicatorNames[i:], ", ")))
			break
		}
		timeout -= expiredTime

		startTime := common.Now()
		desc, apiResp, err := clusterStatApi().GetPipelineIndicatorDesc(
			group, pipelineName, indicatorName, req)
		expiredTime = common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(desc)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func ClusterRetrieveTaskIndicatorNames(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	}

	pipelineNames := args

	var expiredTime time.Duration
	for i, pipelineName := range pipelineNames {
		req := new(pdu.StatisticsClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())

		if timeout <= expiredTime {
			errs.append(fmt.Errorf(
				"timeout: skip to handle [%s]", strings.Join(pipelineNames[i:], ", ")))
			break
		}
		timeout -= expiredTime

		startTime := common.Now()
		retrieveResp, apiResp, err := clusterStatApi().GetTaskIndicatorNames(group, pipelineName, req)
		expiredTime = common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", pipelineName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(retrieveResp)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", pipelineName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func ClusterGetTaskIndicatorValue(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]
	indicatorNames := args[1:]

	var expiredTime time.Duration
	for i, indicatorName := range indicatorNames {
		req := new(pdu.StatisticsClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())

		if timeout <= expiredTime {
			errs.append(fmt.Errorf(
				"timeout: skip to handle [%s]", strings.Join(indicatorNames[i:], ", ")))
			break
		}
		timeout -= expiredTime

		startTime := common.Now()
		value, apiResp, err := clusterStatApi().GetTaskIndicatorValue(
			group, pipelineName, indicatorName, req)
		expiredTime = common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(value)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}

func ClusterGetTaskIndicatorDesc(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second

	errs := &multipleErr{}

	if len(args) < 1 {
		errs.append(fmt.Errorf("pipeline name requied"))
		return errs.Return()
	} else if len(args) < 2 {
		errs.append(fmt.Errorf("indicator name requied"))
		return errs.Return()
	}

	pipelineName := args[0]
	indicatorNames := args[1:]

	var expiredTime time.Duration
	for i, indicatorName := range indicatorNames {
		req := new(pdu.StatisticsClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())

		if timeout <= expiredTime {
			errs.append(fmt.Errorf(
				"timeout: skip to handle [%s]", strings.Join(indicatorNames[i:], ", ")))
			break
		}
		timeout -= expiredTime

		startTime := common.Now()
		desc, apiResp, err := clusterStatApi().GetTaskIndicatorDesc(group, pipelineName, indicatorName, req)
		expiredTime = common.Since(startTime)

		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		} else if apiResp.Error != nil {
			errs.append(fmt.Errorf("%s-%s: %s", pipelineName, indicatorName, apiResp.Error.Error))
			continue
		}

		data, err := json.Marshal(desc)
		if err != nil {
			errs.append(fmt.Errorf("%s-%s: %v", pipelineName, indicatorName, err))
			continue
		}

		// TODO: make it pretty
		fmt.Printf("%s\n", data)
	}

	return errs.Return()
}
