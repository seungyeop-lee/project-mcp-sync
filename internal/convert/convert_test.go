package convert

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/seungyeop-lee/project-mcp-sync/internal/codextoml"
	"github.com/seungyeop-lee/project-mcp-sync/internal/mcpjson"
)

func TestToCodexStdioServer(t *testing.T) {
	set, reason := ToCodex(&mcpjson.Server{
		Command: "npx",
		Args:    []string{"-y", "@upstash/context7-mcp"},
		Env:     map[string]string{"CONTEXT7_API_KEY": "secret"},
	})
	if reason != "" {
		t.Fatalf("reason = %q, want convertible", reason)
	}
	want := map[string]any{
		"command": "npx",
		"args":    []string{"-y", "@upstash/context7-mcp"},
		"env":     map[string]string{"CONTEXT7_API_KEY": "secret"},
	}
	if !reflect.DeepEqual(set, want) {
		t.Errorf("set = %#v, want %#v", set, want)
	}
}

func TestToCodexHTTPServer(t *testing.T) {
	set, reason := ToCodex(&mcpjson.Server{
		Type:    mcpjson.TypeHTTP,
		URL:     "https://mcp.notion.com/mcp",
		Headers: map[string]string{"Notion-Version": "2022-06-28"},
	})
	if reason != "" {
		t.Fatalf("reason = %q, want convertible", reason)
	}
	want := map[string]any{
		"url":          "https://mcp.notion.com/mcp",
		"http_headers": map[string]string{"Notion-Version": "2022-06-28"},
	}
	if !reflect.DeepEqual(set, want) {
		t.Errorf("set = %#v, want %#v", set, want)
	}
}

// alwaysLoad, timeout 같은 Claude-only 필드는 .mcp.json에만 남고 변환 자체는 막지 않는다 (skip 대상은 headersHelper, oauth뿐).
func TestToCodexAllowsHarmlessClaudeOnlyFields(t *testing.T) {
	set, reason := ToCodex(&mcpjson.Server{
		Command: "npx",
		ClaudeOnly: map[string]json.RawMessage{
			"alwaysLoad": []byte(`true`),
			"timeout":    []byte(`3000`),
		},
	})
	if reason != "" {
		t.Fatalf("reason = %q, want convertible", reason)
	}
	if set["command"] != "npx" {
		t.Errorf("set = %#v", set)
	}
}

func TestToCodexSkips(t *testing.T) {
	cases := []struct {
		name   string
		srv    *mcpjson.Server
		wantIn string
	}{
		{
			"ws type",
			&mcpjson.Server{Type: mcpjson.TypeWS, URL: "wss://example.com"},
			"ws",
		},
		{
			"sse type",
			&mcpjson.Server{Type: mcpjson.TypeSSE, URL: "https://example.com/sse"},
			"sse",
		},
		{
			"headersHelper",
			&mcpjson.Server{
				Type:       mcpjson.TypeHTTP,
				URL:        "https://example.com/mcp",
				ClaudeOnly: map[string]json.RawMessage{"headersHelper": []byte(`"helper.sh"`)},
			},
			"headersHelper",
		},
		{
			"oauth",
			&mcpjson.Server{
				Type:       mcpjson.TypeHTTP,
				URL:        "https://example.com/mcp",
				ClaudeOnly: map[string]json.RawMessage{"oauth": []byte(`{}`)},
			},
			"oauth",
		},
		{
			"var in command",
			&mcpjson.Server{Command: "${MCP_BIN}"},
			"command",
		},
		{
			"var in args",
			&mcpjson.Server{Command: "npx", Args: []string{"-y", "${PKG}"}},
			"args",
		},
		{
			"var inserted in url",
			&mcpjson.Server{Type: mcpjson.TypeHTTP, URL: "${API_BASE}/mcp"},
			"url",
		},
		{
			"full var url",
			&mcpjson.Server{Type: mcpjson.TypeHTTP, URL: "${API_URL}"},
			"url",
		},
		{
			"default syntax in env",
			&mcpjson.Server{Command: "npx", Env: map[string]string{"KEY": "${VAR:-default}"}},
			"env",
		},
		{
			"string composition in header",
			&mcpjson.Server{
				Type:    mcpjson.TypeHTTP,
				URL:     "https://example.com/mcp",
				Headers: map[string]string{"X-Key": "prefix-${VAR}-suffix"},
			},
			"header",
		},
		{
			// 동명 passthrough만 매트릭스 대상이다
			"env full var with different name",
			&mcpjson.Server{Command: "npx", Env: map[string]string{"KEY": "${OTHER}"}},
			"env",
		},
		{
			// codex의 bearer_token_env_var는 Authorization 헤더만 만든다
			"bearer in non-authorization header",
			&mcpjson.Server{
				Type:    mcpjson.TypeHTTP,
				URL:     "https://example.com/mcp",
				Headers: map[string]string{"X-Auth": "Bearer ${TOKEN}"},
			},
			"header",
		},
		{
			"bearer pattern in env",
			&mcpjson.Server{Command: "npx", Env: map[string]string{"KEY": "Bearer ${TOKEN}"}},
			"env",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set, reason := ToCodex(tc.srv)
			if set != nil {
				t.Errorf("set = %#v, want nil", set)
			}
			if reason == "" || !strings.Contains(reason, tc.wantIn) {
				t.Errorf("reason = %q, want mention of %q", reason, tc.wantIn)
			}
		})
	}
}

// 안전 변환 매트릭스 패턴은 codex의 env 참조 필드로 변환된다.
// 리터럴 값과 섞여 있어도 매트릭스 패턴만 분리되어야 한다.
func TestToCodexMatrixPatterns(t *testing.T) {
	cases := []struct {
		name string
		srv  *mcpjson.Server
		want map[string]any
	}{
		{
			"bearer token header",
			&mcpjson.Server{
				Type: mcpjson.TypeHTTP,
				URL:  "https://example.com/mcp",
				Headers: map[string]string{
					"Authorization":  "Bearer ${TOKEN}",
					"Notion-Version": "2022-06-28",
				},
			},
			map[string]any{
				"url":                  "https://example.com/mcp",
				"bearer_token_env_var": "TOKEN",
				"http_headers":         map[string]string{"Notion-Version": "2022-06-28"},
			},
		},
		{
			"full var header",
			&mcpjson.Server{
				Type:    mcpjson.TypeHTTP,
				URL:     "https://example.com/mcp",
				Headers: map[string]string{"X-Api-Key": "${API_KEY}"},
			},
			map[string]any{
				"url":              "https://example.com/mcp",
				"env_http_headers": map[string]string{"X-Api-Key": "API_KEY"},
			},
		},
		{
			"same-name env passthrough",
			&mcpjson.Server{
				Command: "npx",
				Env:     map[string]string{"GITHUB_TOKEN": "${GITHUB_TOKEN}", "MODE": "fast"},
			},
			map[string]any{
				"command":  "npx",
				"env":      map[string]string{"MODE": "fast"},
				"env_vars": []string{"GITHUB_TOKEN"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set, reason := ToCodex(tc.srv)
			if reason != "" {
				t.Fatalf("reason = %q, want convertible", reason)
			}
			if !reflect.DeepEqual(set, tc.want) {
				t.Errorf("set = %#v, want %#v", set, tc.want)
			}
		})
	}
}

func TestToMCPJSONStdioServer(t *testing.T) {
	out, reason := ToMCPJSON(&codextoml.Server{
		Command: "npx",
		Args:    []string{"-y", "@upstash/context7-mcp"},
		Env:     map[string]string{"CONTEXT7_API_KEY": "secret"},
	})
	if reason != "" {
		t.Fatalf("reason = %q, want convertible", reason)
	}
	want := &mcpjson.Server{
		Command: "npx",
		Args:    []string{"-y", "@upstash/context7-mcp"},
		Env:     map[string]string{"CONTEXT7_API_KEY": "secret"},
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}
}

func TestToMCPJSONHTTPServer(t *testing.T) {
	out, reason := ToMCPJSON(&codextoml.Server{
		URL:         "https://mcp.notion.com/mcp",
		HTTPHeaders: map[string]string{"Notion-Version": "2022-06-28"},
	})
	if reason != "" {
		t.Fatalf("reason = %q, want convertible", reason)
	}
	want := &mcpjson.Server{
		Type:    mcpjson.TypeHTTP,
		URL:     "https://mcp.notion.com/mcp",
		Headers: map[string]string{"Notion-Version": "2022-06-28"},
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("out = %#v, want %#v", out, want)
	}
}

func TestToMCPJSONSkips(t *testing.T) {
	cases := []struct {
		name string
		srv  *codextoml.Server
	}{
		{"both command and url", &codextoml.Server{Command: "npx", URL: "https://example.com"}},
		{"neither command nor url", &codextoml.Server{}},
		// codex의 리터럴 ${...} 값을 .mcp.json으로 옮기면 Claude가 환경변수로 확장해 의미가 달라지므로 skip한다
		{"literal var in env", &codextoml.Server{Command: "npx", Env: map[string]string{"KEY": "${KEY}"}}},
		{"literal var in header", &codextoml.Server{URL: "https://example.com", HTTPHeaders: map[string]string{"X-Key": "${API_KEY}"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, reason := ToMCPJSON(tc.srv)
			if out != nil || reason == "" {
				t.Errorf("out = %#v, reason = %q, want skip", out, reason)
			}
		})
	}
}

// codex의 env 참조 필드는 .mcp.json의 ${VAR} 패턴으로 복원된다.
// 리터럴 필드(http_headers, env)와 섞여 있으면 한 맵으로 합쳐진다.
func TestToMCPJSONEnvRefFields(t *testing.T) {
	cases := []struct {
		name string
		srv  *codextoml.Server
		want *mcpjson.Server
	}{
		{
			"bearer_token_env_var",
			&codextoml.Server{
				URL:               "https://example.com/mcp",
				BearerTokenEnvVar: "TOKEN",
				HTTPHeaders:       map[string]string{"Notion-Version": "2022-06-28"},
			},
			&mcpjson.Server{
				Type: mcpjson.TypeHTTP,
				URL:  "https://example.com/mcp",
				Headers: map[string]string{
					"Authorization":  "Bearer ${TOKEN}",
					"Notion-Version": "2022-06-28",
				},
			},
		},
		{
			"env_http_headers",
			&codextoml.Server{
				URL:            "https://example.com/mcp",
				EnvHTTPHeaders: map[string]string{"X-Api-Key": "API_KEY"},
			},
			&mcpjson.Server{
				Type:    mcpjson.TypeHTTP,
				URL:     "https://example.com/mcp",
				Headers: map[string]string{"X-Api-Key": "${API_KEY}"},
			},
		},
		{
			"env_vars",
			&codextoml.Server{
				Command: "npx",
				Env:     map[string]string{"MODE": "fast"},
				EnvVars: []string{"GITHUB_TOKEN"},
			},
			&mcpjson.Server{
				Command: "npx",
				Env:     map[string]string{"GITHUB_TOKEN": "${GITHUB_TOKEN}", "MODE": "fast"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, reason := ToMCPJSON(tc.srv)
			if reason != "" {
				t.Fatalf("reason = %q, want convertible", reason)
			}
			if !reflect.DeepEqual(out, tc.want) {
				t.Errorf("out = %#v, want %#v", out, tc.want)
			}
		})
	}
}

// env 참조 필드를 복원할 수 없으면 인증 정보가 조용히 빠진 .mcp.json이 생기므로 skip해야 한다.
func TestToMCPJSONEnvRefFieldSkips(t *testing.T) {
	cases := []struct {
		name string
		srv  *codextoml.Server
	}{
		{
			"bearer_token_env_var on stdio server",
			&codextoml.Server{Command: "npx", BearerTokenEnvVar: "TOKEN"},
		},
		{
			"env_http_headers on stdio server",
			&codextoml.Server{Command: "npx", EnvHTTPHeaders: map[string]string{"X-Key": "API_KEY"}},
		},
		{
			"env_vars on url server",
			&codextoml.Server{URL: "https://example.com", EnvVars: []string{"KEY"}},
		},
		{
			"env_vars entry conflicts with env key",
			&codextoml.Server{Command: "npx", Env: map[string]string{"KEY": "v"}, EnvVars: []string{"KEY"}},
		},
		{
			"env_http_headers key conflicts with http_headers",
			&codextoml.Server{
				URL:            "https://example.com",
				HTTPHeaders:    map[string]string{"X-Key": "literal"},
				EnvHTTPHeaders: map[string]string{"X-Key": "API_KEY"},
			},
		},
		{
			"bearer_token_env_var conflicts with Authorization header",
			&codextoml.Server{
				URL:               "https://example.com",
				HTTPHeaders:       map[string]string{"Authorization": "Basic abc"},
				BearerTokenEnvVar: "TOKEN",
			},
		},
		{
			"invalid env var name",
			&codextoml.Server{URL: "https://example.com", BearerTokenEnvVar: "1BAD"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, reason := ToMCPJSON(tc.srv)
			if out != nil || reason == "" {
				t.Errorf("out = %#v, reason = %q, want skip", out, reason)
			}
		})
	}
}

// source에 더 이상 없는 매트릭스 필드(bearer_token_env_var 등)도 제거 대상에 들어가야 stale한 인증 설정이 codex 테이블에 남지 않는다.
func TestRemoveListCoversAllManagedFieldsNotSet(t *testing.T) {
	remove := RemoveList(map[string]any{
		"url":          "https://example.com",
		"http_headers": map[string]string{"X-Key": "v"},
	})
	sort.Strings(remove)
	want := []string{"args", "bearer_token_env_var", "command", "env", "env_http_headers", "env_vars"}
	if !reflect.DeepEqual(remove, want) {
		t.Errorf("remove = %v, want %v", remove, want)
	}
}
