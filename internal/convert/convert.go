// Package convert는 .mcp.json 서버와 .codex/config.toml 서버 사이의 서버 단위 변환과 변환 불가(skip) 판정을 제공한다.
//
// ${VAR} 안전 패턴은 양방향 매트릭스로 변환한다:
//   - headers의 Authorization "Bearer ${TOKEN}" <-> bearer_token_env_var = "TOKEN"
//   - 헤더 값 전체가 "${VAR}" <-> env_http_headers = { "<헤더>" = "VAR" }
//   - stdio env의 동명 passthrough "KEY": "${KEY}" <-> env_vars = ["KEY"]
//
// 매트릭스 밖 ${...} 패턴(문자열 중간 삽입, ${VAR:-default}, 문자열 조합, command/args/url 내 참조)은 변환하지 않고 skip 사유를 돌려준다.
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
	for _, field := range srv.CodexIncompatibleClaudeOnly() {
		return nil, fmt.Sprintf("%q has no Codex equivalent", field)
	}
	// command/args/url의 ${VAR}는 Codex에 대응 수단이 없어 매트릭스 밖이다.
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
	env, envVars, reason := splitEnv(srv.Env)
	if reason != "" {
		return nil, reason
	}
	headers, envHeaders, bearerVar, reason := splitHeaders(srv.Headers)
	if reason != "" {
		return nil, reason
	}

	set = map[string]any{}
	if srv.Command != "" {
		set["command"] = srv.Command
	}
	if len(srv.Args) > 0 {
		set["args"] = srv.Args
	}
	if len(env) > 0 {
		set["env"] = env
	}
	if len(envVars) > 0 {
		set["env_vars"] = envVars
	}
	if srv.URL != "" {
		set["url"] = srv.URL
	}
	if bearerVar != "" {
		set["bearer_token_env_var"] = bearerVar
	}
	if len(headers) > 0 {
		set["http_headers"] = headers
	}
	if len(envHeaders) > 0 {
		set["env_http_headers"] = envHeaders
	}
	return set, ""
}

// ToMCPJSON은 codex 서버 하나를 .mcp.json 서버로 변환한다.
// 변환할 수 없으면 서버는 nil이고 reason에 skip 사유가 담긴다.
func ToMCPJSON(srv *codextoml.Server) (out *mcpjson.Server, reason string) {
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

	// env 참조 필드를 복원할 수 없으면 인증 정보가 조용히 빠진 .mcp.json이 생기므로 skip이 안전하다.
	// 서버 종류 불일치, 충돌, 잘못된 변수명이 여기 해당한다.
	if hasCommand {
		if srv.BearerTokenEnvVar != "" || len(srv.EnvHTTPHeaders) > 0 {
			return nil, "stdio server has HTTP-only env-reference fields (bearer_token_env_var, env_http_headers)"
		}
		env, reason := mergeEnvVars(srv.Env, srv.EnvVars)
		if reason != "" {
			return nil, reason
		}
		return &mcpjson.Server{Command: srv.Command, Args: srv.Args, Env: env}, ""
	}

	if len(srv.EnvVars) > 0 {
		return nil, "url server has env_vars, which only applies to stdio servers"
	}
	headers, reason := mergeHeaders(srv.HTTPHeaders, srv.EnvHTTPHeaders, srv.BearerTokenEnvVar)
	if reason != "" {
		return nil, reason
	}
	return &mcpjson.Server{Type: mcpjson.TypeHTTP, URL: srv.URL, Headers: headers}, ""
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

// sync가 소유하는 codex 테이블 필드.
// 여기 없는 필드(enabled, timeout류 등)는 Codex-only로 간주해 건드리지 않는다.
var managedFields = []string{
	"command", "args", "env", "env_vars",
	"url", "bearer_token_env_var", "http_headers", "env_http_headers",
}

// splitEnv는 .mcp.json env를 리터럴 값(env)과 동명 passthrough(env_vars)로 나눈다.
func splitEnv(in map[string]string) (env map[string]string, envVars []string, reason string) {
	for _, key := range sortedKeys(in) {
		value := in[key]
		ref := envref.Classify(value)
		switch {
		case ref.Kind == envref.None:
			if env == nil {
				env = map[string]string{}
			}
			env[key] = value
		case ref.Kind == envref.FullVar && ref.Var == key:
			envVars = append(envVars, key)
		default:
			return nil, nil, fmt.Sprintf("env %s value %q contains a ${...} reference outside the conversion matrix", key, value)
		}
	}
	return env, envVars, ""
}

// splitHeaders는 .mcp.json headers를 리터럴(http_headers), 값 전체 변수 참조(env_http_headers), Authorization bearer 변수(bearer_token_env_var)로 나눈다.
// "Bearer ${VAR}"는 Authorization 헤더에서만 변환할 수 있다.
// codex의 bearer_token_env_var가 Authorization 헤더만 만들기 때문이다.
func splitHeaders(in map[string]string) (headers, envHeaders map[string]string, bearerVar, reason string) {
	for _, key := range sortedKeys(in) {
		value := in[key]
		ref := envref.Classify(value)
		switch {
		case ref.Kind == envref.None:
			if headers == nil {
				headers = map[string]string{}
			}
			headers[key] = value
		case ref.Kind == envref.BearerToken && key == "Authorization":
			bearerVar = ref.Var
		case ref.Kind == envref.FullVar:
			if envHeaders == nil {
				envHeaders = map[string]string{}
			}
			envHeaders[key] = ref.Var
		default:
			return nil, nil, "", fmt.Sprintf("header %s value %q contains a ${...} reference outside the conversion matrix", key, value)
		}
	}
	return headers, envHeaders, bearerVar, ""
}

// mergeEnvVars는 codex env_vars의 각 항목을 동명 passthrough("KEY": "${KEY}")로 복원해 env에 합친다.
// 같은 키를 두 곳에서 정의하면 어느 쪽이 이길지 알 수 없으므로 skip한다.
func mergeEnvVars(env map[string]string, envVars []string) (map[string]string, string) {
	if len(envVars) == 0 {
		return env, ""
	}
	out := map[string]string{}
	for key, value := range env {
		out[key] = value
	}
	for _, name := range envVars {
		if !envref.ValidName(name) {
			return nil, fmt.Sprintf("env_vars entry %q is not a valid environment variable name", name)
		}
		if _, ok := out[name]; ok {
			return nil, fmt.Sprintf("env_vars entry %q conflicts with an env key", name)
		}
		out[name] = "${" + name + "}"
	}
	return out, ""
}

// mergeHeaders는 codex의 env 참조 필드를 ${VAR} 형태로 복원해 http_headers에 합친다.
// 같은 헤더를 두 곳에서 정의하면 어느 쪽이 이길지 알 수 없으므로 skip한다.
func mergeHeaders(httpHeaders, envHeaders map[string]string, bearerVar string) (map[string]string, string) {
	if len(envHeaders) == 0 && bearerVar == "" {
		return httpHeaders, ""
	}
	out := map[string]string{}
	for key, value := range httpHeaders {
		out[key] = value
	}
	for _, key := range sortedKeys(envHeaders) {
		name := envHeaders[key]
		if !envref.ValidName(name) {
			return nil, fmt.Sprintf("env_http_headers value %q is not a valid environment variable name", name)
		}
		if _, ok := out[key]; ok {
			return nil, fmt.Sprintf("env_http_headers key %q conflicts with http_headers", key)
		}
		out[key] = "${" + name + "}"
	}
	if bearerVar != "" {
		if !envref.ValidName(bearerVar) {
			return nil, fmt.Sprintf("bearer_token_env_var %q is not a valid environment variable name", bearerVar)
		}
		if _, ok := out["Authorization"]; ok {
			return nil, `bearer_token_env_var conflicts with an "Authorization" header`
		}
		out["Authorization"] = "Bearer ${" + bearerVar + "}"
	}
	return out, ""
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
