package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "completion [zsh|bash|fish]",
		Short:                 "Generate shell completion script",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"zsh", "bash", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			out := cmd.OutOrStdout()
			switch args[0] {
			case "zsh":
				return root.GenZshCompletion(out)
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "fish":
				return root.GenFishCompletion(out, true)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
}
