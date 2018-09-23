package cli

import (
	"encoding/json"
	"fmt"

	"github.com/hexdecteam/easegateway/pkg/common"

	"github.com/hexdecteam/easegateway-go-client/rest/1.0/common/v1"
	cpdu "github.com/hexdecteam/easegateway-go-client/rest/1.0/common/v1/pdu"
	"github.com/urfave/cli"
)

func ClusterHealthCheck(c *cli.Context) error {
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	req := new(cpdu.ClusterRequest)
	req.TimeoutSec = timeoutSec

	resp, apiResp, err := clusterHealthApi().GetGroupsHealth(req)

	return printClusterResult(resp, apiResp, err)
}

func ClusterGroupHealthCheck(c *cli.Context) error {
	args := c.Args()
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	req := new(cpdu.ClusterRequest)
	req.TimeoutSec = timeoutSec
	if len(args) < 1 {
		return fmt.Errorf("group name required")
	}
	group := args[0]
	resp, apiResp, err := clusterHealthApi().GetGroupHealth(group, req)

	return printClusterResult(resp, apiResp, err)
}

func ClusterGetGroups(c *cli.Context) error {
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	req := new(cpdu.ClusterRequest)
	req.TimeoutSec = timeoutSec

	resp, apiResp, err := clusterHealthApi().GetGroups(req)

	return printClusterResult(resp, apiResp, err)
}

func ClusterGetGroup(c *cli.Context) error {
	args := c.Args()
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	req := new(cpdu.ClusterRequest)
	req.TimeoutSec = timeoutSec
	if len(args) < 1 {
		return fmt.Errorf("group name required")
	}
	group := args[0]

	resp, apiResp, err := clusterHealthApi().GetGroup(group, req)

	return printClusterResult(resp, apiResp, err)
}

func ClusterGetMembers(c *cli.Context) error {
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	req := new(cpdu.ClusterRequest)
	req.TimeoutSec = timeoutSec
	group := c.String("group")
	if group == "" {
		return fmt.Errorf("group can't be empty")
	}
	resp, apiResp, err := clusterHealthApi().GetMembers(group, req)

	return printClusterResult(resp, apiResp, err)
}

func ClusterGetMember(c *cli.Context) error {
	args := c.Args()
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	req := new(cpdu.ClusterRequest)
	req.TimeoutSec = timeoutSec
	group := c.String("group")
	if group == "" {
		return fmt.Errorf("group can't be empty")
	}
	if len(args) < 1 {
		return fmt.Errorf("member name required")
	}
	member := args[0]

	resp, apiResp, err := clusterHealthApi().GetMember(group, member, req)

	return printClusterResult(resp, apiResp, err)
}

func printClusterResult(resp interface{}, apiResp *v1.APIResponse, err error) error {
	errs := &multipleErr{}

	if err != nil {
		errs.append(err)
		return errs.Return()
	} else if apiResp.Error != nil {
		errs.append(fmt.Errorf("%s", apiResp.Error.Error))
		return errs.Return()
	}

	data, err := json.Marshal(resp)
	if err != nil {
		errs.append(err)
		return errs.Return()
	}

	fmt.Printf("%s\n", data)
	return nil
}
