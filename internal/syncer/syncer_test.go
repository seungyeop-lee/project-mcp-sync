package syncer

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seungyeop-lee/project-mcp-sync/internal/mcpjson"
)

func TestSyncUpdatesCodexFromMCPJSON(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json":          "mcp_basic.json",
		".codex/config.toml": "codex_basic.toml",
	})
	res, err := Run(root, SourceAuto, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed() {
		t.Error("Changed = false, want true")
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
	// notion은 codex에 없던 서버, context7은 값이 달라진 서버, legacy는 source에서 사라진 서버다
	if !reflect.DeepEqual(res.Adds, []string{"notion"}) {
		t.Errorf("Adds = %v, want [notion]", res.Adds)
	}
	if !reflect.DeepEqual(res.Updates, []string{"context7"}) {
		t.Errorf("Updates = %v, want [context7]", res.Updates)
	}
	if !reflect.DeepEqual(res.Deletes, []string{"legacy"}) {
		t.Errorf("Deletes = %v, want [legacy]", res.Deletes)
	}
	checkGolden(t, "codex_after_update.toml", readFile(t, root, ".codex/config.toml"))

	// .mcp.json은 source이므로 수정되지 않는다
	if !bytes.Equal(readFile(t, root, ".mcp.json"), readFixture(t, "mcp_basic.json")) {
		t.Error(".mcp.json must not be modified in mcp-to-codex direction")
	}

	// 같은 입력으로 다시 실행하면 변경이 없어야 한다 (멱등성)
	res2, err := Run(root, SourceAuto, false)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Changed() {
		t.Error("second run must report Changed = false")
	}
}

func TestSyncCreatesCodexWhenMissing(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json": "mcp_basic.json",
	})
	res, err := Run(root, SourceAuto, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed() {
		t.Error("Changed = false, want true")
	}
	checkGolden(t, "codex_created.toml", readFile(t, root, ".codex/config.toml"))
}

func TestSyncGeneratesMCPJSONFromCodex(t *testing.T) {
	root := setupProject(t, map[string]string{
		".codex/config.toml": "codex_basic.toml",
	})
	res, err := Run(root, SourceAuto, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed() {
		t.Error("Changed = false, want true")
	}
	checkGolden(t, "mcp_generated.json", readFile(t, root, ".mcp.json"))

	// codex가 source인 방향에서는 codex 파일이 byte 단위로 보존되어야 한다
	if !bytes.Equal(readFile(t, root, ".codex/config.toml"), readFixture(t, "codex_basic.toml")) {
		t.Error(".codex/config.toml must not be modified in codex-to-mcp direction")
	}
}

// --source codex는 .mcp.json이 이미 있어도 codex를 source로 강제해 .mcp.json을 갱신한다.
// 갱신 시 직교 Claude-only 필드(alwaysLoad, timeout)와 Unknown 필드는 보존되고, 겹침 필드(oauth)는 제거된다.
func TestSyncSourceCodexUpdatesMCPJSON(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json":          "mcp_reverse.json",
		".codex/config.toml": "codex_reverse.toml",
	})
	res, err := Run(root, SourceCodex, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed() {
		t.Error("Changed = false, want true")
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
	// notion은 .mcp.json에 없던 서버, context7은 값이 달라진 서버, legacy는 source에서 사라진 서버다
	if !reflect.DeepEqual(res.Adds, []string{"notion"}) {
		t.Errorf("Adds = %v, want [notion]", res.Adds)
	}
	if !reflect.DeepEqual(res.Updates, []string{"context7"}) {
		t.Errorf("Updates = %v, want [context7]", res.Updates)
	}
	if !reflect.DeepEqual(res.Deletes, []string{"legacy"}) {
		t.Errorf("Deletes = %v, want [legacy]", res.Deletes)
	}
	checkGolden(t, "mcp_after_update.json", readFile(t, root, ".mcp.json"))

	// codex는 source이므로 수정되지 않는다
	if !bytes.Equal(readFile(t, root, ".codex/config.toml"), readFixture(t, "codex_reverse.toml")) {
		t.Error(".codex/config.toml must not be modified when codex is the source")
	}

	// 같은 입력으로 다시 실행하면 변경이 없어야 한다 (멱등성)
	res2, err := Run(root, SourceCodex, false)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Changed() {
		t.Error("second run must report Changed = false")
	}
}

// 변환 불가한 codex 서버는 skip되고, 동명의 기존 .mcp.json 서버는 건드리지 않는다.
func TestSyncSourceCodexSkipsUnconvertibleServers(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json":          "mcp_reverse_skip.json",
		".codex/config.toml": "codex_reverse_skip.toml",
	})
	res, err := Run(root, SourceCodex, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "envy") {
		t.Errorf("Warnings = %v, want one warning mentioning envy", res.Warnings)
	}
	// skip한 envy의 기존 정의는 그대로, good만 추가된다
	checkGolden(t, "mcp_after_reverse_skip.json", readFile(t, root, ".mcp.json"))
}

// --source codex이고 .mcp.json이 없으면 자동 판별과 같은 생성 모드로 동작한다.
func TestSyncSourceCodexCreatesWhenMCPJSONMissing(t *testing.T) {
	root := setupProject(t, map[string]string{
		".codex/config.toml": "codex_basic.toml",
	})
	if _, err := Run(root, SourceCodex, false); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "mcp_generated.json", readFile(t, root, ".mcp.json"))
}

// --source mcp-json은 양쪽이 다 있을 때 자동 판별과 같은 정방향으로 동작한다.
func TestSyncSourceMCPJSONForcesForward(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json":          "mcp_basic.json",
		".codex/config.toml": "codex_basic.toml",
	})
	res, err := Run(root, SourceMCPJSON, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.File != ".codex/config.toml" {
		t.Errorf("File = %q, want .codex/config.toml", res.File)
	}
	checkGolden(t, "codex_after_update.toml", readFile(t, root, ".codex/config.toml"))
}

// 강제한 source 파일이 없으면 에러다.
func TestSyncSourceErrorsWhenForcedSourceMissing(t *testing.T) {
	t.Run("source mcp-json without .mcp.json", func(t *testing.T) {
		root := setupProject(t, map[string]string{
			".codex/config.toml": "codex_basic.toml",
		})
		if _, err := Run(root, SourceMCPJSON, false); err == nil {
			t.Fatal("Run should fail when --source mcp-json is forced but .mcp.json does not exist")
		}
	})
	t.Run("source codex without config.toml", func(t *testing.T) {
		root := setupProject(t, map[string]string{
			".mcp.json": "mcp_basic.json",
		})
		if _, err := Run(root, SourceCodex, false); err == nil {
			t.Fatal("Run should fail when --source codex is forced but .codex/config.toml does not exist")
		}
	})
}

// 의미 변화가 없으면 기존 .mcp.json의 포맷(키 순서·들여쓰기)을 다시 쓰지 않는다.
func TestSyncSourceCodexNoSemanticChangeKeepsBytes(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, ".codex/config.toml", []byte("[mcp_servers.alpha]\ncommand = \"alpha-mcp\"\n"))
	// Marshal 결과와 다른 압축 포맷. 의미가 같으므로 그대로 남아야 한다.
	mcp := []byte("{\"mcpServers\":{\"alpha\":{\"command\":\"alpha-mcp\"}}}\n")
	writeProjectFile(t, root, ".mcp.json", mcp)
	res, err := Run(root, SourceCodex, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed() {
		t.Error("Changed = true, want false for a semantically identical .mcp.json")
	}
	if !bytes.Equal(readFile(t, root, ".mcp.json"), mcp) {
		t.Error(".mcp.json bytes must be preserved when nothing changes semantically")
	}
}

// 빈 .mcp.json도 source of truth다.
// mcpServers가 없거나 {}이면 codex의 MCP 테이블을 모두 비우되 비-MCP 설정은 남긴다.
func TestSyncEmptyMCPJSONClearsCodexServers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{"no mcpServers key", "{}\n"},
		{"empty mcpServers", "{\n  \"mcpServers\": {}\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := setupProject(t, map[string]string{
				".codex/config.toml": "codex_basic.toml",
			})
			writeProjectFile(t, root, ".mcp.json", []byte(tc.content))
			res, err := Run(root, SourceAuto, false)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Changed() {
				t.Error("Changed = false, want true")
			}
			checkGolden(t, "codex_after_empty.toml", readFile(t, root, ".codex/config.toml"))
		})
	}
}

func TestSyncErrorsWhenBothMissing(t *testing.T) {
	if _, err := Run(t.TempDir(), SourceAuto, false); err == nil {
		t.Fatal("Run should fail when neither file exists")
	}
}

func TestSyncSkipsUnconvertibleServers(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json":          "mcp_skip.json",
		".codex/config.toml": "codex_skip.toml",
	})
	res, err := Run(root, SourceAuto, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "envy") {
		t.Errorf("Warnings = %v, want one warning mentioning envy", res.Warnings)
	}
	// skip한 envy의 기존 테이블(주석 포함)은 그대로, good만 추가된다
	checkGolden(t, "codex_after_skip.toml", readFile(t, root, ".codex/config.toml"))
}

// ${VAR} 안전 패턴(Authorization bearer, 전체 변수 헤더, 동명 env passthrough)은 codex의 env 참조 필드(bearer_token_env_var, env_http_headers, env_vars)로 변환된다.
func TestSyncMatrixPatternsToCodex(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json": "mcp_matrix.json",
	})
	res, err := Run(root, SourceAuto, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
	checkGolden(t, "codex_matrix_created.toml", readFile(t, root, ".codex/config.toml"))
}

func TestSyncMatrixPatternsToMCPJSON(t *testing.T) {
	root := setupProject(t, map[string]string{
		".codex/config.toml": "codex_matrix.toml",
	})
	res, err := Run(root, SourceAuto, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
	checkGolden(t, "mcp_matrix_generated.json", readFile(t, root, ".mcp.json"))
}

// .mcp.json -> .codex/config.toml -> .mcp.json round-trip에서 매트릭스 패턴의 의미가 보존되는지 확인한다.
// 비교는 byte가 아니라 파싱된 모델로 한다.
func TestSyncMatrixRoundTrip(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json": "mcp_matrix.json",
	})
	if _, err := Run(root, SourceAuto, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".mcp.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, SourceAuto, false); err != nil {
		t.Fatal(err)
	}

	got, err := mcpjson.Parse(readFile(t, root, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := mcpjson.Parse(readFixture(t, "mcp_matrix.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip changed meaning\n got = %#v\nwant = %#v", got, want)
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	root := setupProject(t, map[string]string{
		".mcp.json":          "mcp_basic.json",
		".codex/config.toml": "codex_basic.toml",
	})
	res, err := Run(root, SourceAuto, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed() {
		t.Error("dry-run must still report Changed = true")
	}
	if !bytes.Equal(readFile(t, root, ".codex/config.toml"), readFixture(t, "codex_basic.toml")) {
		t.Error("dry-run must not modify .codex/config.toml")
	}
}

func TestSyncParseErrors(t *testing.T) {
	t.Run("invalid mcp json", func(t *testing.T) {
		root := t.TempDir()
		writeProjectFile(t, root, ".mcp.json", []byte("{oops"))
		if _, err := Run(root, SourceAuto, false); err == nil {
			t.Fatal("Run should fail on invalid .mcp.json")
		}
	})
	t.Run("invalid codex toml", func(t *testing.T) {
		root := setupProject(t, map[string]string{
			".mcp.json": "mcp_basic.json",
		})
		writeProjectFile(t, root, ".codex/config.toml", []byte("="))
		if _, err := Run(root, SourceAuto, false); err == nil {
			t.Fatal("Run should fail on invalid .codex/config.toml")
		}
	})
}

func setupProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, fixture := range files {
		writeProjectFile(t, root, rel, readFixture(t, fixture))
	}
	return root
}

func writeProjectFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return data
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

var update = flag.Bool("update", false, "rewrite golden files")

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test -update to generate)", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output does not match golden %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
