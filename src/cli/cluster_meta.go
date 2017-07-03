package cli

import "github.com/urfave/cli"

// TODO: add clusterMetaServer to support these APIs.

// url: /cluster/meta/v1/groups
func ClusterGroups(c *cli.Context) error {
	return nil
}

// TODO: display details(write/read mode) of every member if set
// url: /cluster/meta/v1/members
func ClusterMembers(c *cli.Context) error {
	return nil
}

// TODO: display details(write/read mode) of every member if set
// url: /cluster/meta/v1/groups/test_group/members
func ClusterGroupMembers(c *cli.Context) error {
	return nil
}

// TODO: move healthCheckServer.info to clusterMetaServer.info?
// url: /cluster/meta/v1/groups/test_group/members/node-1/info
func ClusterMemberInfo(c *cli.Context) error {
	return nil
}
