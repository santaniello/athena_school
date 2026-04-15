package main

import (
	"fmt"

	"github.com/fsantaniello/athena_school/internal/platform/config"
	"github.com/spf13/cobra"
)

func newConfigCmd(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage athena configuration",
	}
	cmd.AddCommand(newConfigGetCmd(configPath))
	cmd.AddCommand(newConfigSetCmd(configPath))
	return cmd
}

func newConfigGetCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Print the current configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(*configPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "provider: %s\nmodel:    %s\n", cfg.Provider, cfg.Model)
			return nil
		},
	}
}

func newConfigSetCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			cfg, err := config.Load(*configPath)
			if err != nil {
				return err
			}

			switch key {
			case "provider":
				cfg.Provider = value
			case "model":
				cfg.Model = value
			case "ollama.host":
				cfg.Ollama.Host = value
			default:
				return fmt.Errorf("unknown config key %q", key)
			}

			if err := config.Save(*configPath, cfg); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ %s set to %q\n", key, value)
			return nil
		},
	}
}
