package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seungyeop-lee/project-mcp-sync/cmd/commands"
)

// drift가 있으면 unified diff를 출력하고 ErrDriftDetected를 돌려줘야
// main이 exit 1로 매핑한다
func TestDiffReportsDriftWithUnifiedDiff(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".mcp.json", `{"mcpServers": {"good": {"command": "npx"}}}`)

	out, err := execute(t, "diff", "--project", dir)
	if !errors.Is(err, commands.ErrDriftDetected) {
		t.Fatalf("err = %v, want ErrDriftDetected", err)
	}
	for _, want := range []string{"--- /dev/null", "+++ b/.codex/config.toml", "@@", "+[mcp_servers.good]"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff output missing %q\noutput:\n%s", want, out)
		}
	}
	// diff는 검사 전용이므로 파일을 쓰지 않는다
	if _, err := os.Stat(filepath.Join(dir, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Error("diff must not create config.toml")
	}
}

func TestDiffExitsZeroWhenInSync(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".mcp.json", `{"mcpServers": {"good": {"command": "npx"}}}`)
	if _, err := executeSync(t, "sync", "--project", dir); err != nil {
		t.Fatal(err)
	}

	out, err := execute(t, "diff", "--project", dir)
	if err != nil {
		t.Fatalf("diff after sync must report no drift: %v", err)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

// 둘 다 없을 때는 drift(1)가 아니라 오류(2)다
func TestDiffFailsWhenNoConfigExists(t *testing.T) {
	_, err := execute(t, "diff", "--project", t.TempDir())
	if err == nil || errors.Is(err, commands.ErrDriftDetected) {
		t.Fatalf("err = %v, want non-drift error", err)
	}
}
