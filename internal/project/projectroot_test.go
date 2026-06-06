package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRootFromNestedDir(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	nested := filepath.Join(root, "a", "b")
	mustMkdirAll(t, nested)

	got, err := FindProjectRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("FindProjectRoot = %q, want %q", got, root)
	}
}

// git worktree에서는 .git이 디렉토리가 아니라 파일이다
func TestFindProjectRootWithGitFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := FindProjectRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("FindProjectRoot = %q, want %q", got, root)
	}
}

func TestFindProjectRootReturnsNearest(t *testing.T) {
	outer := t.TempDir()
	mustMkdirAll(t, filepath.Join(outer, ".git"))
	inner := filepath.Join(outer, "sub")
	mustMkdirAll(t, filepath.Join(inner, ".git"))
	start := filepath.Join(inner, "x")
	mustMkdirAll(t, start)

	got, err := FindProjectRoot(start)
	if err != nil {
		t.Fatal(err)
	}
	if got != inner {
		t.Errorf("FindProjectRoot = %q, want %q", got, inner)
	}
}

// .git을 filesystem root까지 못 찾으면 에러 대신 start 자체를 project root로 사용한다 (cwd fallback)
func TestFindProjectRootFallsBackToStart(t *testing.T) {
	start := t.TempDir()
	got, err := FindProjectRoot(start)
	if err != nil {
		t.Fatal(err)
	}
	if got != start {
		t.Errorf("FindProjectRoot = %q, want %q", got, start)
	}
}

// fallback도 상대경로 입력을 절대경로로 돌려줘야 한다
func TestFindProjectRootFallbackReturnsAbsolutePath(t *testing.T) {
	t.Chdir(t.TempDir())

	got, err := FindProjectRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("FindProjectRoot = %q, want absolute path", got)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
