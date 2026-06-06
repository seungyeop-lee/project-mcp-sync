package main

import (
	"errors"
	"os"

	"github.com/seungyeop-lee/project-mcp-sync/cmd"
	"github.com/seungyeop-lee/project-mcp-sync/cmd/commands"
)

func main() {
	// exit code 정책: 0=성공, 2=오류. diff command의 drift만 1로 매핑한다.
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, commands.ErrDriftDetected) {
			os.Exit(1)
		}
		os.Exit(2)
	}
}
