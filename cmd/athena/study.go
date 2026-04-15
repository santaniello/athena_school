package main

import (
	"context"
	"os"

	"github.com/fsantaniello/athena_school/internal/platform/config"
	"github.com/fsantaniello/athena_school/internal/platform/llm"
	"github.com/fsantaniello/athena_school/internal/study"
	"github.com/spf13/cobra"
)

func newStudyCmd(configPath *string) *cobra.Command {
	var providerFlag string
	var modelFlag string

	cmd := &cobra.Command{
		Use:   "study <topic> [subtopic]",
		Short: "Start an interactive study session on a topic",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := args[0]
			subtopic := ""
			if len(args) > 1 {
				subtopic = args[1]
			}

			cfg, err := config.Load(*configPath)
			if err != nil {
				return err
			}

			provider := cfg.Provider
			if providerFlag != "" {
				provider = providerFlag
			}
			model := cfg.Model
			if modelFlag != "" {
				model = modelFlag
			}

			llmProvider, err := llm.NewProvider(provider, cfg.Ollama.Host)
			if err != nil {
				return err
			}

			session := study.NewSession(topic, subtopic, model, llmProvider, os.Stdin, cmd.OutOrStdout())
			return session.Run(context.Background())
		},
	}

	cmd.Flags().StringVar(&providerFlag, "provider", "", "LLM provider to use (overrides config)")
	cmd.Flags().StringVar(&modelFlag, "model", "", "model name to use (overrides config)")
	return cmd
}
