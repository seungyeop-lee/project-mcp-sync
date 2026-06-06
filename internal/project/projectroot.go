// Package project는 project root 탐지를 제공한다.
package project

import (
	"os"
	"path/filepath"
)

// FindProjectRoot는 start에서 부모로 올라가며 .git(디렉토리 또는 worktree의
// .git 파일)이 있는 가장 가까운 디렉토리를 돌려준다.
// filesystem root까지 .git이 없으면 start의 절대경로를 project root로 사용한다.
// 잘못된 디렉토리가 잡혀도 .mcp.json과 .codex/config.toml 둘 다 없으면 sync가
// 에러를 내므로 그 정책이 안전망 역할을 한다.
func FindProjectRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for dir := abs; ; {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs, nil
		}
		dir = parent
	}
}
