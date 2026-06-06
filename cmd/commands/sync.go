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
	source  string
}

func NewSyncCmd() *cobra.Command {
	opts := &syncOptions{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync MCP server definitions between .mcp.json and .codex/config.toml",
		Long:  "Sync MCP server definitions between .mcp.json and .codex/config.toml.\n\n.mcp.json is the source of truth when present; if it does not exist, .codex/config.toml is used as the source instead. If neither file exists, sync fails.\nUse --source to force the source of truth instead of auto-detection; the forced source file must exist.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := parseSource(opts.source)
			if err != nil {
				return err
			}
			root, err := resolveProjectRoot(opts.project)
			if err != nil {
				return err
			}
			plan, err := syncer.Run(root, source, opts.dryRun)
			if err != nil {
				return err
			}
			if opts.dryRun {
				// dry-run 요약이 skip 사유까지 보여주므로 stderr warning은 따로 내보내지 않는다.
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
	addSourceFlag(cmd, &opts.source)

	return cmd
}

// printDryRunSummary는 사람이 읽는 미리보기를 출력한다.
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

func addSourceFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "source", "", `force the source of truth: "mcp-json" or "codex" (default: auto-detect by file presence)`)
}

func parseSource(value string) (syncer.Source, error) {
	switch source := syncer.Source(value); source {
	case syncer.SourceAuto, syncer.SourceMCPJSON, syncer.SourceCodex:
		return source, nil
	default:
		return "", fmt.Errorf(`invalid --source value %q (valid values: "mcp-json", "codex")`, value)
	}
}

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
