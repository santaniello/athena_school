package main

import (
	"io"
	"os"

	"github.com/fsantaniello/athena_school/internal/platform/config"
	"github.com/spf13/cobra"
)

func buildRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "athena",
		Short: "Athena — active learning CLI for developers",
	}

	configPath := new(string)
	cmd.PersistentFlags().StringVar(configPath, "config", "", "path to config file (default: ~/.config/athena/config.yaml)")
	if err := cmd.PersistentFlags().MarkHidden("config"); err != nil {
		panic(err)
	}

	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if *configPath == "" {
			defaultPath, err := config.DefaultPath()
			if err != nil {
				return err
			}
			*configPath = defaultPath
		}
		return nil
	}

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newConfigCmd(configPath))
	cmd.AddCommand(newStudyCmd(configPath))
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