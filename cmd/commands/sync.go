package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/seungyeop-lee/project-mcp-sync/internal/project"
	"github.com/seungyeop-lee/project-mcp-sync/internal/syncer"
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
			root, err := resolveProjectRoot(opts.project)
			if err != nil {
				return err
			}
			res, err := syncer.Run(root, opts.dryRun)
			if err != nil {
				return err
			}
			for _, warning := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+warning)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "show what would change without writing files")
	cmd.Flags().StringVar(&opts.project, "project", "", "project root directory (default: nearest directory containing .git)")

	return cmd
}

// resolveProjectRoot는 --project가 지정되면 그대로 쓰고, 아니면 cwd에서 .git을
// 탐색해 project root를 찾는다. diff command도 같은 규칙을 쓴다.
func resolveProjectRoot(projectFlag string) (string, error) {
	if projectFlag != "" {
		return projectFlag, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return project.FindProjectRoot(cwd)
}
