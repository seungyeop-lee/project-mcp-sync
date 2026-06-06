package codextoml

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseBasic(t *testing.T) {
	d := parseFixture(t, "basic.toml")
	if want := []string{"context7", "github"}; !reflect.DeepEqual(d.Names(), want) {
		t.Fatalf("Names() = %v, want %v", d.Names(), want)
	}

	c7 := d.Server("context7")
	if c7.Command != "npx" {
		t.Errorf("Command = %q, want npx", c7.Command)
	}
	if want := []string{"-y", "@upstash/context7-mcp"}; !reflect.DeepEqual(c7.Args, want) {
		t.Errorf("Args = %v, want %v", c7.Args, want)
	}
	if c7.Env["CONTEXT7_API_KEY"] != "secret" {
		t.Errorf("Env = %v", c7.Env)
	}
	if c7.Other["enabled"] != true {
		t.Errorf("Other[enabled] = %v, want true", c7.Other["enabled"])
	}
	if c7.Other["startup_timeout_sec"] != int64(20) {
		t.Errorf("Other[startup_timeout_sec] = %v, want 20", c7.Other["startup_timeout_sec"])
	}

	gh := d.Server("github")
	if gh.URL != "https://api.githubcopilot.com/mcp/" {
		t.Errorf("URL = %q", gh.URL)
	}
	if gh.BearerTokenEnvVar != "GITHUB_PAT" {
		t.Errorf("BearerTokenEnvVar = %q", gh.BearerTokenEnvVar)
	}
	if gh.Other["custom_field"] != "keep-me" {
		t.Errorf("Other[custom_field] = %v", gh.Other["custom_field"])
	}
}

func TestBytesWithoutPatchIsIdentical(t *testing.T) {
	original := readFixture(t, "basic.toml")
	d, err := Parse(original)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d.Bytes(), original) {
		t.Error("Bytes() must return the original input when nothing was patched")
	}
}

func TestUpsertUpdatesCoreFieldsOnly(t *testing.T) {
	original := readFixture(t, "basic.toml")
	d, err := Parse(original)
	if err != nil {
		t.Fatal(err)
	}
	err = d.Upsert("context7", map[string]any{
		"command": "bunx",
		"args":    []string{"-y", "context7-mcp@2.0"},
		"env":     map[string]string{"CONTEXT7_API_KEY": "rotated"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := d.Bytes()
	checkGolden(t, "update_core.toml", got)

	// Codex-only 필드는 같은 테이블에 남아야 한다
	c7 := d.Server("context7")
	if c7.Other["enabled"] != true || c7.Other["startup_timeout_sec"] != int64(20) {
		t.Errorf("codex-only fields lost: %v", c7.Other)
	}

	// MCP 외 영역(파일 앞부분과 [projects] section)은 byte 단위로 보존되어야 한다
	prefix := original[:bytes.Index(original, []byte("# docs lookup server"))]
	if !bytes.HasPrefix(got, prefix) {
		t.Error("non-MCP prefix region was modified")
	}
	suffix := original[bytes.Index(original, []byte("[projects.")):]
	if !bytes.HasSuffix(got, suffix) {
		t.Error("non-MCP suffix region was modified")
	}
}

func TestUpsertAddsNewServer(t *testing.T) {
	d := parseFixture(t, "basic.toml")
	err := d.Upsert("notion", map[string]any{
		"url":                  "https://mcp.notion.com/mcp",
		"bearer_token_env_var": "NOTION_TOKEN",
		"http_headers":         map[string]string{"Notion-Version": "2022-06-28"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "add_server.toml", d.Bytes())

	if d.Server("notion") == nil {
		t.Fatal("notion not present after upsert")
	}
}

func TestUpsertRemovesFields(t *testing.T) {
	d := parseFixture(t, "basic.toml")
	err := d.Upsert("github", map[string]any{
		"url": "https://api.githubcopilot.com/mcp/",
	}, []string{"bearer_token_env_var"})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "remove_field.toml", d.Bytes())

	gh := d.Server("github")
	if gh.BearerTokenEnvVar != "" {
		t.Errorf("bearer_token_env_var not removed: %q", gh.BearerTokenEnvVar)
	}
	if gh.Other["custom_field"] != "keep-me" {
		t.Errorf("unknown field must survive: %v", gh.Other)
	}
}

func TestDeleteServer(t *testing.T) {
	d := parseFixture(t, "basic.toml")
	if err := d.Delete("context7"); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "delete_server.toml", d.Bytes())

	if d.Server("context7") != nil {
		t.Error("context7 still present after delete")
	}
	if d.Server("github") == nil {
		t.Error("github must survive delete of context7")
	}
}

func TestDeleteMissingServerIsNoop(t *testing.T) {
	original := readFixture(t, "basic.toml")
	d, err := Parse(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Delete("nope"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d.Bytes(), original) {
		t.Error("delete of missing server must not change the file")
	}
}

func TestUpsertCreatesNewFile(t *testing.T) {
	d, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	err = d.Upsert("context7", map[string]any{
		"command": "npx",
		"args":    []string{"-y", "@upstash/context7-mcp"},
		"env":     map[string]string{"CONTEXT7_API_KEY": "secret"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `[mcp_servers.context7]
command = "npx"
args = ["-y", "@upstash/context7-mcp"]
env = { CONTEXT7_API_KEY = "secret" }
`
	if got := string(d.Bytes()); got != want {
		t.Errorf("new file content mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// dotted key(env.A = ...)와 sub-table([mcp_servers.x.http_headers]) 표기도 코어 필드 갱신 시 inline table 한 줄로 합쳐져야 한다.
func TestUpsertNormalizesDottedAndSubTable(t *testing.T) {
	src := `[mcp_servers.x]
command = "a"
env.A = "1"
env.B = "2"

[mcp_servers.x.http_headers]
Authorization = "old"
`
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	err = d.Upsert("x", map[string]any{
		"env":          map[string]string{"C": "3"},
		"http_headers": map[string]string{"X-Key": "y"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `[mcp_servers.x]
command = "a"
env = { C = "3" }
http_headers = { X-Key = "y" }
`
	if got := string(d.Bytes()); got != want {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"invalid toml", "="},
		{"mcp_servers not a table", "mcp_servers = 3"},
		{"env not a table", "[mcp_servers.x]\nenv = \"oops\""},
		{"args element not string", "[mcp_servers.x]\nargs = [1]"},
	}
	for _, tc := range cases {
		if _, err := Parse([]byte(tc.data)); err == nil {
			t.Errorf("%s: Parse should fail", tc.name)
		}
	}
}

func TestQuotedServerName(t *testing.T) {
	d, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Upsert("my server", map[string]any{"command": "x"}, nil); err != nil {
		t.Fatal(err)
	}
	want := "[mcp_servers.\"my server\"]\ncommand = \"x\"\n"
	if got := string(d.Bytes()); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if d.Server("my server") == nil {
		t.Error("quoted server name not parsed back")
	}
}

func parseFixture(t *testing.T, name string) *Document {
	t.Helper()
	d, err := Parse(readFixture(t, name))
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return d
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
