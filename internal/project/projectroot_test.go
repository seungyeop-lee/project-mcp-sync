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

func TestFindProjectRootNotFound(t *testing.T) {
	if _, err := FindProjectRoot(t.TempDir()); err == nil {
		t.Fatal("FindProjectRoot should fail when no .git exists in any parent")
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
