package commands

import (
	"errors"

	"github.com/spf13/cobra"
)

type diffOptions struct {
	project string
}

func NewDiffCmd() *cobra.Command {
	opts := &diffOptions{}

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Report drift between .mcp.json and .codex/config.toml without writing files",
		Long:  "Report drift between .mcp.json and .codex/config.toml without writing files.\n\nExit codes: 0 = no drift, 1 = drift exists, 2 = error. Intended for CI and pre-commit checks.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// diff 본체는 후속 이슈에서 구현한다.
			return errors.New("diff is not implemented yet")
		},
	}

	cmd.Flags().StringVar(&opts.project, "project", "", "project root directory (default: nearest directory containing .git)")

	return cmd
}
