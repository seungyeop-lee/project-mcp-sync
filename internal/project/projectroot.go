// Package project는 project root 탐지를 제공한다.
package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindProjectRoot는 start에서 부모로 올라가며 .git(디렉토리 또는 worktree의
// .git 파일)이 있는 가장 가까운 디렉토리를 돌려준다.
func FindProjectRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root not found: no .git in %s or any parent directory (use --project)", start)
		}
		dir = parent
	}
}
