package command

import (
	"github.com/spf13/cobra"
)

// MeshCmd defines mesh command.
func MeshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mesh",
		Short: "deploy and manager mesh components",
	}
	cmd.AddCommand(serviceCmd())
	cmd.AddCommand(tenantCmd())
	return cmd
}



