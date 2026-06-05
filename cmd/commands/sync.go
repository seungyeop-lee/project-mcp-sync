package commands

import (
	"errors"

	"github.com/spf13/cobra"
)

type syncOptions struct {
	dryRun  bool
	project string
}

func NewSyncCmd() *cobra.Command {
	opts := &syncOptions{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync MCP server definitions between .mcp.json and .codex/config.toml",
		Long:  "Sync MCP server definitions between .mcp.json and .codex/config.toml.\n\n.mcp.json is the source of truth when present; if it does not exist, .codex/config.toml is used as the source instead. If neither file exists, sync fails.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// sync 본체는 후속 이슈에서 구현한다.
			return errors.New("sync is not implemented yet")
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "show what would change without writing files")
	cmd.Flags().StringVar(&opts.project, "project", "", "project root directory (default: nearest directory containing .git)")

	return cmd
}
