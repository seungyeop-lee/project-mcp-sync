package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/seungyeop-lee/project-mcp-sync/internal/version"
)

func TestRootCommandSurface(t *testing.T) {
	root := NewRootCmd()
	var visible []string
	for _, c := range root.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		visible = append(visible, c.Name())
	}
	want := []string{"completion", "diff", "sync"}
	if len(visible) != len(want) {
		t.Fatalf("visible commands = %v, want %v", visible, want)
	}
	for i := range want {
		if visible[i] != want[i] {
			t.Fatalf("visible commands = %v, want %v", visible, want)
		}
	}
}

func TestVersionFlagPrintsVersion(t *testing.T) {
	out, err := execute(t, "--version")
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(out, version.Version) {
		t.Errorf("--version output %q does not contain %s", out, version.Version)
	}
}

func TestSyncHelpExposesFlags(t *testing.T) {
	out, err := execute(t, "sync", "--help")
	if err != nil {
		t.Fatalf("sync --help failed: %v", err)
	}
	for _, flag := range []string{"--dry-run", "--project"} {
		if !strings.Contains(out, flag) {
			t.Errorf("sync --help output missing %s", flag)
		}
	}
	// diff는 별도 command다. sync에 flag로 남아 있으면 결정 위반
	if strings.Contains(out, "--diff") {
		t.Error("sync --help must not expose --diff")
	}
}

func TestDiffHelpExposesFlags(t *testing.T) {
	out, err := execute(t, "diff", "--help")
	if err != nil {
		t.Fatalf("diff --help failed: %v", err)
	}
	if !strings.Contains(out, "--project") {
		t.Error("diff --help output missing --project")
	}
}

func TestCompletionGeneratesScript(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		t.Run(shell, func(t *testing.T) {
			out, err := execute(t, "completion", shell)
			if err != nil {
				t.Fatalf("completion %s failed: %v", shell, err)
			}
			if !strings.Contains(out, "project-mcp-sync") {
				t.Errorf("completion %s output does not reference binary name", shell)
			}
		})
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	if _, err := execute(t, "completion", "powershell"); err == nil {
		t.Fatal("completion powershell should fail")
	}
}

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}
