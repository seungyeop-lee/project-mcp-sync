package commands

import (
	"errors"
	"fmt"

	udiff "github.com/aymanbagabas/go-udiff"
	"github.com/spf13/cobra"

	"github.com/seungyeop-lee/project-mcp-sync/internal/syncer"
)

// ErrDriftDetected는 diff command가 drift를 찾았을 때 돌려주는 에러다.
// main이 exit code 1로 매핑한다. 에러라기보다 검사 결과이므로 메시지는 출력하지 않는다.
var ErrDriftDetected = errors.New("drift detected")

type diffOptions struct {
	project string
}

func NewDiffCmd() *cobra.Command {
	opts := &diffOptions{}

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Report drift between .mcp.json and .codex/config.toml without writing files",
		Long:  "Report drift between .mcp.json and .codex/config.toml without writing files.\n\nPrints a unified diff of the file that sync would change.\nExit codes: 0 = no drift, 1 = drift exists, 2 = error. Intended for CI and pre-commit checks.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveProjectRoot(opts.project)
			if err != nil {
				return err
			}
			plan, err := syncer.Compute(root)
			if err != nil {
				return err
			}
			for _, warning := range plan.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+warning)
			}
			if !plan.Changed() {
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), unifiedDiff(plan))
			// drift는 unified diff로 이미 보고됐다.
			// cobra의 "Error: ..." 출력을 막고 exit code 전달용 에러만 돌려준다.
			cmd.SilenceErrors = true
			return ErrDriftDetected
		},
	}

	cmd.Flags().StringVar(&opts.project, "project", "", "project root directory (default: nearest directory containing .git, falling back to the current directory)")

	return cmd
}

func unifiedDiff(plan *syncer.Plan) string {
	oldLabel := "a/" + plan.File
	if plan.Old == nil {
		// 파일 생성은 git diff 관례대로 /dev/null을 원본 label로 쓴다
		oldLabel = "/dev/null"
	}
	return udiff.Unified(oldLabel, "b/"+plan.File, string(plan.Old), string(plan.New))
}
