package main

import (
	"io"
	"os"

	"github.com/spf13/cobra"
)

func buildRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "athena",
		Short: "Athena — active learning CLI for developers",
	}
	cmd.AddCommand(newVersionCmd())
	return cmd
}

func execute(args []string, out io.Writer) error {
	cmd := buildRootCmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func Execute() {
	if err := execute(os.Args[1:], os.Stdout); err != nil {
		os.Exit(1)
	}
}