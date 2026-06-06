// Package convert는 .mcp.json 서버와 .codex/config.toml 서버 사이의
// 서버 단위 변환과 변환 불가(skip) 판정을 제공한다.
//
// ${VAR} 안전 패턴(bearer token, env passthrough 등)의 매트릭스 변환은 아직
// 없으므로 ${...}를 포함한 값과 codex의 env 참조 필드는 모두 skip 대상이다.
package convert

import (
	"fmt"
	"sort"

	"github.com/seungyeop-lee/project-mcp-sync/internal/codextoml"
	"github.com/seungyeop-lee/project-mcp-sync/internal/envref"
	"github.com/seungyeop-lee/project-mcp-sync/internal/mcpjson"
)

// ToCodex는 .mcp.json 서버 하나를 [mcp_servers.<name>] 테이블 필드로 변환한다.
// 변환할 수 없으면 set은 nil이고 reason에 skip 사유가 담긴다.
func ToCodex(srv *mcpjson.Server) (set map[string]any, reason string) {
	switch srv.EffectiveType() {
	case mcpjson.TypeStdio, mcpjson.TypeHTTP:
	default:
		return nil, fmt.Sprintf("type %q is not supported in Codex", srv.EffectiveType())
	}
	for _, field := range []string{"headersHelper", "oauth"} {
		if _, ok := srv.ClaudeOnly[field]; ok {
			return nil, fmt.Sprintf("%q has no Codex equivalent", field)
		}
	}
	if reason := refReason("command", srv.Command); reason != "" {
		return nil, reason
	}
	for _, arg := range srv.Args {
		if reason := refReason("args", arg); reason != "" {
			return nil, reason
		}
	}
	if reason := refReason("url", srv.URL); reason != "" {
		return nil, reason
	}
	for _, key := range sortedKeys(srv.Env) {
		if reason := refReason("env "+key, srv.Env[key]); reason != "" {
			return nil, reason
		}
	}
	for _, key := range sortedKeys(srv.Headers) {
		if reason := refReason("header "+key, srv.Headers[key]); reason != "" {
			return nil, reason
		}
	}

	set = map[string]any{}
	if srv.Command != "" {
		set["command"] = srv.Command
	}
	if len(srv.Args) > 0 {
		set["args"] = srv.Args
	}
	if len(srv.Env) > 0 {
		set["env"] = srv.Env
	}
	if srv.URL != "" {
		set["url"] = srv.URL
	}
	if len(srv.Headers) > 0 {
		set["http_headers"] = srv.Headers
	}
	return set, ""
}

// ToMCPJSON은 codex 서버 하나를 .mcp.json 서버로 변환한다.
// 변환할 수 없으면 서버는 nil이고 reason에 skip 사유가 담긴다.
func ToMCPJSON(srv *codextoml.Server) (out *mcpjson.Server, reason string) {
	// 변환하지 않으면 인증 정보가 조용히 빠진 .mcp.json이 생기므로 skip이 안전하다
	if srv.BearerTokenEnvVar != "" || len(srv.EnvHTTPHeaders) > 0 || len(srv.EnvVars) > 0 {
		return nil, "env-reference fields (bearer_token_env_var, env_http_headers, env_vars) cannot be converted yet"
	}
	hasCommand := srv.Command != ""
	hasURL := srv.URL != ""
	switch {
	case hasCommand && hasURL:
		return nil, "has both command and url"
	case !hasCommand && !hasURL:
		return nil, "has neither command nor url"
	}
	// codex의 ${...}는 리터럴이지만 .mcp.json에서는 Claude가 환경변수로 확장한다.
	// 의미가 달라지므로 옮기지 않는다.
	if reason := refReason("command", srv.Command); reason != "" {
		return nil, reason
	}
	for _, arg := range srv.Args {
		if reason := refReason("args", arg); reason != "" {
			return nil, reason
		}
	}
	if reason := refReason("url", srv.URL); reason != "" {
		return nil, reason
	}
	for _, key := range sortedKeys(srv.Env) {
		if reason := refReason("env "+key, srv.Env[key]); reason != "" {
			return nil, reason
		}
	}
	for _, key := range sortedKeys(srv.HTTPHeaders) {
		if reason := refReason("header "+key, srv.HTTPHeaders[key]); reason != "" {
			return nil, reason
		}
	}

	if hasCommand {
		return &mcpjson.Server{Command: srv.Command, Args: srv.Args, Env: srv.Env}, ""
	}
	return &mcpjson.Server{Type: mcpjson.TypeHTTP, URL: srv.URL, Headers: srv.HTTPHeaders}, ""
}

// RemoveList는 sync가 관리하는 codex 필드 중 set에 없는 것들을 돌려준다.
// Upsert의 remove 인자로 사용해, source에서 사라진 코어 필드를 테이블에서 걷어낸다.
func RemoveList(set map[string]any) []string {
	var remove []string
	for _, field := range managedFields {
		if _, ok := set[field]; !ok {
			remove = append(remove, field)
		}
	}
	return remove
}

// sync가 소유하는 codex 테이블 필드. 여기 없는 필드(enabled, timeout류 등)는
// Codex-only로 간주해 건드리지 않는다.
var managedFields = []string{
	"command", "args", "env", "env_vars",
	"url", "bearer_token_env_var", "http_headers", "env_http_headers",
}

func refReason(field, value string) string {
	if envref.Classify(value).Kind == envref.None {
		return ""
	}
	return fmt.Sprintf("%s value %q contains a ${...} reference that cannot be converted", field, value)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
