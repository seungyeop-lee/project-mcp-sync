package commands

import (
	"fmt"
	"io"
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
			plan, err := syncer.Run(root, opts.dryRun)
			if err != nil {
				return err
			}
			if opts.dryRun {
				printDryRunSummary(cmd.OutOrStdout(), plan)
				return nil
			}
			for _, warning := range plan.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+warning)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "show what would change without writing files")
	cmd.Flags().StringVar(&opts.project, "project", "", "project root directory (default: nearest directory containing .git, falling back to the current directory)")

	return cmd
}

// printDryRunSummary는 사람이 읽는 미리보기를 출력한다. skip 사유까지 한 곳에
// 모아 보여주므로 dry-run에서는 stderr warning을 따로 내보내지 않는다.
func printDryRunSummary(w io.Writer, plan *syncer.Plan) {
	if !plan.Changed() {
		fmt.Fprintf(w, "%s is up to date\n", plan.File)
	} else {
		fmt.Fprintf(w, "would update %s:\n", plan.File)
		for _, name := range plan.Adds {
			fmt.Fprintf(w, "  add    %s\n", name)
		}
		for _, name := range plan.Updates {
			fmt.Fprintf(w, "  update %s\n", name)
		}
		for _, name := range plan.Deletes {
			fmt.Fprintf(w, "  delete %s\n", name)
		}
	}
	if len(plan.Warnings) > 0 {
		fmt.Fprintln(w, "warnings:")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(w, "  %s\n", warning)
		}
	}
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
