package cmd

import (
	"github.com/spf13/cobra"

	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/ann"
	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/intercept"
)

func interceptCmd() *cobra.Command {
	ic := &intercept.Command{}
	cmd := &cobra.Command{
		Use:   "intercept [flags] <name> [-- [[docker run flags] <image name>] OR [<command>]] args...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Intercept a service",
		Annotations: map[string]string{
			ann.Session:           ann.Required,
			ann.UpdateCheckFormat: ann.Tel2,
		},
		SilenceUsage:      true,
		SilenceErrors:     true,
		RunE:              ic.Run,
		ValidArgsFunction: intercept.ValidArgs,
	}
	ic.AddInterceptFlags(cmd)
	return cmd
}
