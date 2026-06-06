package cmd

import (
	"github.com/spf13/cobra"

	"github.com/seungyeop-lee/project-mcp-sync/cmd/commands"
	"github.com/seungyeop-lee/project-mcp-sync/internal/version"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "project-mcp-sync",
		Short:         "Sync project-scoped MCP server definitions between Claude Code and Codex",
		Long:          "Sync project-scoped MCP server definitions between Claude Code's .mcp.json and Codex's .codex/config.toml.",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	// Cobra 기본 completion command는 powershell까지 노출하므로 비활성화하고 zsh/bash/fish만 받는 custom command를 등록한다.
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.AddCommand(commands.NewSyncCmd())
	rootCmd.AddCommand(commands.NewDiffCmd())
	rootCmd.AddCommand(commands.NewCompletionCmd())

	return rootCmd
}

func Execute() error {
	return NewRootCmd().Execute()
}
