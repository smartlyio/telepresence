package cmd

import (
	"github.com/spf13/cobra"

	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/ann"
	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/intercept"
)

func wiretapCmd() *cobra.Command {
	ic := &intercept.Command{
		Wiretap: true,
	}
	cmd := &cobra.Command{
		Use:   "wiretap [flags] <wiretap_base_name> [-- <command with arguments...>]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Wiretap a Service",
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
