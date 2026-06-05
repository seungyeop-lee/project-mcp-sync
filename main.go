package main

import (
	"os"

	"github.com/seungyeop-lee/project-mcp-sync/cmd"
)

func main() {
	// exit code 정책: 0=성공, 2=오류. (sync --diff의 1=drift는 해당 command 구현에서 처리)
	if err := cmd.Execute(); err != nil {
		os.Exit(2)
	}
}
