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
