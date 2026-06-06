package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 성공 시 에러 없이 끝나야 main이 exit 0으로 매핑한다
func TestSyncCreatesCodexConfig(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".mcp.json", `{"mcpServers": {"good": {"command": "npx"}}}`)

	if _, err := executeSync(t, "sync", "--project", dir); err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("config.toml not created: %v", err)
	}
	if !strings.Contains(string(data), "[mcp_servers.good]") {
		t.Errorf("config.toml content = %q", data)
	}
}

// 둘 다 없으면 에러를 돌려줘야 main이 exit 2로 매핑한다
func TestSyncFailsWhenNoConfigExists(t *testing.T) {
	if _, err := executeSync(t, "sync", "--project", t.TempDir()); err == nil {
		t.Fatal("sync should fail when neither config exists")
	}
}

func TestSyncFailsOnParseError(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".mcp.json", "{oops")

	if _, err := executeSync(t, "sync", "--project", dir); err == nil {
		t.Fatal("sync should fail on invalid .mcp.json")
	}
}

func TestSyncPrintsSkipWarningsToStderr(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".mcp.json", `{"mcpServers": {"envy": {"type": "http", "url": "${API_BASE}/mcp"}}}`)

	stderr, err := executeSync(t, "sync", "--project", dir)
	if err != nil {
		t.Fatalf("skip must not fail the command: %v", err)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "envy") {
		t.Errorf("stderr = %q, want warning mentioning envy", stderr)
	}
}

func TestSyncDryRunWritesNoFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".mcp.json", `{"mcpServers": {"good": {"command": "npx"}}}`)

	if _, err := executeSync(t, "sync", "--dry-run", "--project", dir); err != nil {
		t.Fatalf("sync --dry-run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Error("dry-run must not create config.toml")
	}
}

// drift가 있어도 dry-run은 검사 모드가 아니므로 성공(exit 0)이고,
// 변경 요약과 skip warning을 stdout으로 출력한다
func TestSyncDryRunPrintsSummary(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".mcp.json",
		`{"mcpServers": {"good": {"command": "npx"}, "envy": {"type": "http", "url": "${API_BASE}/mcp"}}}`)

	out, err := execute(t, "sync", "--dry-run", "--project", dir)
	if err != nil {
		t.Fatalf("dry-run must succeed even when drift exists: %v", err)
	}
	for _, want := range []string{"would update .codex/config.toml", "add    good", "envy"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q\noutput:\n%s", want, out)
		}
	}
}

func TestSyncDryRunReportsUpToDate(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".mcp.json", `{"mcpServers": {"good": {"command": "npx"}}}`)
	if _, err := executeSync(t, "sync", "--project", dir); err != nil {
		t.Fatal(err)
	}

	out, err := execute(t, "sync", "--dry-run", "--project", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("output = %q, want up-to-date message", out)
	}
}

// executeSync는 stderr를 돌려준다. warning 출력 검증용.
func executeSync(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	errBuf := &bytes.Buffer{}
	root.SetOut(errBuf)
	root.SetErr(errBuf)
	root.SetArgs(args)
	err := root.Execute()
	return errBuf.String(), err
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
