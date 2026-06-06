package mcpjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseStdio(t *testing.T) {
	f := parseFixture(t, "stdio.json")
	srv, ok := f.Servers["context7"]
	if !ok {
		t.Fatalf("server context7 not found, servers = %v", f.Names())
	}
	if srv.EffectiveType() != TypeStdio {
		t.Errorf("EffectiveType = %q, want stdio", srv.EffectiveType())
	}
	if srv.Type != "" {
		t.Errorf("Type = %q, want empty (type field absent in fixture)", srv.Type)
	}
	if srv.Command != "npx" {
		t.Errorf("Command = %q, want npx", srv.Command)
	}
	if want := []string{"-y", "@upstash/context7-mcp"}; !reflect.DeepEqual(srv.Args, want) {
		t.Errorf("Args = %v, want %v", srv.Args, want)
	}
	if srv.Env["CONTEXT7_API_KEY"] != "${CONTEXT7_API_KEY}" {
		t.Errorf("Env = %v, want CONTEXT7_API_KEY passthrough", srv.Env)
	}
}

func TestParseHTTPHeaders(t *testing.T) {
	f := parseFixture(t, "http_headers.json")
	srv := f.Servers["github"]
	if srv == nil {
		t.Fatal("server github not found")
	}
	if srv.Type != TypeHTTP {
		t.Errorf("Type = %q, want http", srv.Type)
	}
	if srv.URL != "https://api.githubcopilot.com/mcp/" {
		t.Errorf("URL = %q", srv.URL)
	}
	want := map[string]string{
		"Authorization": "Bearer ${GITHUB_PAT}",
		"X-Tenant":      "${TENANT_ID}",
	}
	if !reflect.DeepEqual(srv.Headers, want) {
		t.Errorf("Headers = %v, want %v", srv.Headers, want)
	}
}

func TestParseSSEAndWS(t *testing.T) {
	cases := []struct {
		fixture string
		server  string
		typ     string
	}{
		{"sse.json", "legacy-sse", TypeSSE},
		{"ws.json", "realtime", TypeWS},
	}
	for _, tc := range cases {
		f := parseFixture(t, tc.fixture)
		srv := f.Servers[tc.server]
		if srv == nil {
			t.Fatalf("%s: server %s not found", tc.fixture, tc.server)
		}
		if srv.Type != tc.typ {
			t.Errorf("%s: Type = %q, want %q", tc.fixture, srv.Type, tc.typ)
		}
	}
}

func TestParseClaudeOnlyFields(t *testing.T) {
	cases := []struct {
		fixture string
		server  string
		fields  []string
	}{
		{"oauth.json", "asana", []string{"oauth", "alwaysLoad", "timeout"}},
		{"headers_helper.json", "internal-api", []string{"headersHelper"}},
	}
	for _, tc := range cases {
		f := parseFixture(t, tc.fixture)
		srv := f.Servers[tc.server]
		if srv == nil {
			t.Fatalf("%s: server %s not found", tc.fixture, tc.server)
		}
		for _, field := range tc.fields {
			if _, ok := srv.ClaudeOnly[field]; !ok {
				t.Errorf("%s: ClaudeOnly missing %q, got %v", tc.fixture, field, srv.ClaudeOnly)
			}
		}
		if len(srv.Unknown) != 0 {
			t.Errorf("%s: Claude-only fields must not land in Unknown: %v", tc.fixture, srv.Unknown)
		}
	}
}

func TestParseEmptyAndAbsentServers(t *testing.T) {
	for _, fixture := range []string{"empty_servers.json", "no_servers_key.json"} {
		f := parseFixture(t, fixture)
		if len(f.Servers) != 0 {
			t.Errorf("%s: Servers = %v, want empty", fixture, f.Names())
		}
	}
}

func TestParseEmptyInput(t *testing.T) {
	f, err := Parse([]byte("  \n"))
	if err != nil {
		t.Fatalf("Parse(whitespace) error: %v", err)
	}
	if len(f.Servers) != 0 {
		t.Errorf("Servers = %v, want empty", f.Names())
	}
}

func TestParseUnknownFieldsNotError(t *testing.T) {
	f := parseFixture(t, "unknown_fields.json")
	if _, ok := f.Extra["$schema"]; !ok {
		t.Errorf("top-level unknown field $schema not preserved: %v", f.Extra)
	}
	srv := f.Servers["future-server"]
	if srv == nil {
		t.Fatal("server future-server not found")
	}
	for _, field := range []string{"experimentalFlag", "nested"} {
		if _, ok := srv.Unknown[field]; !ok {
			t.Errorf("Unknown missing %q, got %v", field, srv.Unknown)
		}
	}
	if srv.Command != "future-mcp" {
		t.Errorf("Command = %q, want future-mcp", srv.Command)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"invalid json", "{"},
		{"mcpServers is array", `{"mcpServers": []}`},
		{"server def is string", `{"mcpServers": {"a": "npx"}}`},
		{"command is number", `{"mcpServers": {"a": {"command": 1}}}`},
		{"args is string", `{"mcpServers": {"a": {"args": "-y"}}}`},
		{"env value is number", `{"mcpServers": {"a": {"env": {"K": 1}}}}`},
	}
	for _, tc := range cases {
		if _, err := Parse([]byte(tc.data)); err == nil {
			t.Errorf("%s: Parse should fail", tc.name)
		}
	}
}

// parse -> generate -> parse 결과가 처음 parse 결과와 같아야 한다 (의미 보존).
func TestRoundTrip(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			first := parseFixture(t, e.Name())
			out, err := first.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			second, err := Parse(out)
			if err != nil {
				t.Fatalf("re-parse generated output: %v\n%s", err, out)
			}
			if !reflect.DeepEqual(first, second) {
				t.Errorf("round-trip mismatch\nfirst:  %+v\nsecond: %+v\noutput:\n%s", first, second, out)
			}
		})
	}
}

func TestMarshalEmptyFile(t *testing.T) {
	f := &File{}
	out, err := f.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	reparsed, err := Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v\n%s", err, out)
	}
	if len(reparsed.Servers) != 0 {
		t.Errorf("Servers = %v, want empty", reparsed.Names())
	}
}

func TestNamesSorted(t *testing.T) {
	f := &File{Servers: map[string]*Server{
		"zeta":  {},
		"alpha": {},
		"mid":   {},
	}}
	want := []string{"alpha", "mid", "zeta"}
	if got := f.Names(); !reflect.DeepEqual(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
}

// 동명 서버 merge: 코어 필드는 src로 교체, 직교 필드(alwaysLoad, timeout)와 Unknown은 보존, 겹침 필드(oauth, headersHelper)는 제거.
func TestUpsertMergesExistingServer(t *testing.T) {
	f := &File{Servers: map[string]*Server{
		"api": {
			Type:    TypeHTTP,
			URL:     "https://old.example.com/mcp",
			Headers: map[string]string{"X-Old": "1"},
			ClaudeOnly: map[string]json.RawMessage{
				"oauth":         json.RawMessage(`{"clientId":"abc"}`),
				"headersHelper": json.RawMessage(`"helper.sh"`),
				"alwaysLoad":    json.RawMessage(`true`),
				"timeout":       json.RawMessage(`30000`),
			},
			Unknown: map[string]json.RawMessage{"experimentalFlag": json.RawMessage(`true`)},
		},
	}}
	f.Upsert("api", &Server{
		Type:    TypeHTTP,
		URL:     "https://new.example.com/mcp",
		Headers: map[string]string{"Authorization": "Bearer ${TOKEN}"},
	})

	want := &Server{
		Type:    TypeHTTP,
		URL:     "https://new.example.com/mcp",
		Headers: map[string]string{"Authorization": "Bearer ${TOKEN}"},
		ClaudeOnly: map[string]json.RawMessage{
			"alwaysLoad": json.RawMessage(`true`),
			"timeout":    json.RawMessage(`30000`),
		},
		Unknown: map[string]json.RawMessage{"experimentalFlag": json.RawMessage(`true`)},
	}
	if got := f.Servers["api"]; !reflect.DeepEqual(got, want) {
		t.Errorf("Upsert merged server mismatch\n got = %+v\nwant = %+v", got, want)
	}
}

func TestUpsertAddsNewServer(t *testing.T) {
	f := &File{Servers: map[string]*Server{}}
	src := &Server{Command: "npx", Args: []string{"-y", "new-mcp"}}
	f.Upsert("fresh", src)
	if got := f.Servers["fresh"]; !reflect.DeepEqual(got, src) {
		t.Errorf("Upsert added server mismatch\n got = %+v\nwant = %+v", got, src)
	}
}

// 보존 대상 Claude-only 필드가 없으면 ClaudeOnly는 nil로 정규화되어 Parse 결과 모델과 일치해야 한다.
func TestUpsertNormalizesEmptyClaudeOnly(t *testing.T) {
	f := &File{Servers: map[string]*Server{
		"api": {
			Command:    "old-mcp",
			ClaudeOnly: map[string]json.RawMessage{"oauth": json.RawMessage(`{}`)},
		},
	}}
	f.Upsert("api", &Server{Command: "new-mcp"})
	if got := f.Servers["api"].ClaudeOnly; got != nil {
		t.Errorf("ClaudeOnly = %v, want nil", got)
	}
}

func TestDeleteRemovesWholeServer(t *testing.T) {
	f := &File{Servers: map[string]*Server{
		"gone": {Command: "x", ClaudeOnly: map[string]json.RawMessage{"alwaysLoad": json.RawMessage(`true`)}},
	}}
	f.Delete("gone")
	if len(f.Servers) != 0 {
		t.Errorf("Servers = %v, want empty", f.Names())
	}
	// 없는 서버 삭제는 no-op이다
	f.Delete("absent")
}

func parseFixture(t *testing.T, name string) *File {
	t.Helper()
	f, err := Parse(readFixture(t, name))
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return f
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}
