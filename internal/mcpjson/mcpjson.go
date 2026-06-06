// Package mcpjson은 Claude Code의 .mcp.json을 내부 모델로 파싱하고, 내부 모델에서 .mcp.json을 생성한다.
package mcpjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// 서버 type 값. type 필드가 없으면 stdio로 해석한다.
const (
	TypeStdio = "stdio"
	TypeHTTP  = "http"
	TypeSSE   = "sse"
	TypeWS    = "ws"
)

// Claude Code에서만 의미 있는 서버 필드의 분류 테이블.
// 파싱 시 ClaudeOnly로 모으는 기준이자, ToCodex의 skip 판정(CodexIncompatibleClaudeOnly)과 codex가 source일 때 Upsert의 보존/제거 판정이 모두 이 테이블 하나를 본다.
var claudeOnlyFields = map[string]mergePolicy{
	// 코어 필드(headers 등)와 같은 영역(인증·헤더 구성)을 다루는 대체 수단. source 기준으로 덮어쓴 코어 필드와 공존하면 의미가 충돌하므로 merge 시 제거한다.
	"headersHelper": policyRemove,
	"oauth":         policyRemove,
	// 코어 필드와 직교하는 메타데이터. source가 표현할 수 없는 정보이므로 merge 시 보존한다.
	"alwaysLoad": policyPreserve,
	"timeout":    policyPreserve,
}

// mergePolicy는 codex가 source일 때 Upsert가 기존 서버의 Claude-only 필드를 어떻게 다룰지 정한다.
type mergePolicy int

const (
	policyRemove mergePolicy = iota
	policyPreserve
)

type File struct {
	Servers map[string]*Server
	// mcpServers 외의 최상위 필드. round-trip 시 그대로 보존한다.
	Extra map[string]json.RawMessage
}

type Server struct {
	// Type은 파일에 적힌 값 그대로다.
	Type    string
	Command string
	Args    []string
	Env     map[string]string
	URL     string
	Headers map[string]string
	// ClaudeOnly는 claudeOnlyFields 테이블에 분류된 필드다.
	ClaudeOnly map[string]json.RawMessage
	// Unknown은 코어/Claude-only 어느 쪽도 아닌 필드다. 파싱 에러로 만들지 않고 보존한다.
	Unknown map[string]json.RawMessage
}

func (s *Server) EffectiveType() string {
	if s.Type == "" {
		return TypeStdio
	}
	return s.Type
}

// CodexIncompatibleClaudeOnly는 서버가 가진 Claude-only 필드 중 Codex에 대응 수단이 없어 변환을 막는 필드 이름을 정렬해 돌려준다.
// 코어 필드와 영역이 겹치는 필드(policyRemove)가 곧 변환을 막는 필드다.
func (s *Server) CodexIncompatibleClaudeOnly() []string {
	var fields []string
	for key := range s.ClaudeOnly {
		if claudeOnlyFields[key] == policyRemove {
			fields = append(fields, key)
		}
	}
	sort.Strings(fields)
	return fields
}

// Names는 서버 이름을 정렬해 돌려준다. map 순회 순서에 의존하지 않기 위한 helper.
func (f *File) Names() []string {
	names := make([]string, 0, len(f.Servers))
	for name := range f.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Upsert는 codex가 source일 때의 서버 단위 merge다 (codextoml.Document.Upsert의 거울상).
// 동명 서버가 없으면 src를 그대로 추가한다.
// 있으면 코어 필드는 src로 교체하고, 기존 서버의 Claude-only 필드는 claudeOnlyFields 분류에 따라 직교 필드만 보존하며, Unknown 필드는 보존한다.
func (f *File) Upsert(name string, src *Server) {
	existing := f.Servers[name]
	if existing == nil {
		f.Servers[name] = src
		return
	}
	merged := *src
	merged.ClaudeOnly = map[string]json.RawMessage{}
	for key, raw := range src.ClaudeOnly {
		merged.ClaudeOnly[key] = raw
	}
	for key, raw := range existing.ClaudeOnly {
		if claudeOnlyFields[key] == policyPreserve {
			merged.ClaudeOnly[key] = raw
		}
	}
	// 빈 map은 부재로 정규화해 Parse 결과 모델과의 비교(DeepEqual)를 깨지 않는다.
	if len(merged.ClaudeOnly) == 0 {
		merged.ClaudeOnly = nil
	}
	merged.Unknown = existing.Unknown
	f.Servers[name] = &merged
}

// Delete는 서버 정의를 직교 Claude-only 필드까지 통째로 제거한다. 서버가 없으면 아무것도 하지 않는다.
func (f *File) Delete(name string) {
	delete(f.Servers, name)
}

func Parse(data []byte) (*File, error) {
	f := &File{
		Servers: map[string]*Server{},
		Extra:   map[string]json.RawMessage{},
	}
	// 내용이 전혀 없는 .mcp.json은 mcpServers가 없는 것과 같게 취급한다.
	if len(bytes.TrimSpace(data)) == 0 {
		return f, nil
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parse .mcp.json: %w", err)
	}

	for key, raw := range top {
		if key != "mcpServers" {
			f.Extra[key] = compactRaw(raw)
			continue
		}
		var servers map[string]json.RawMessage
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, fmt.Errorf("parse .mcp.json: mcpServers must be an object: %w", err)
		}
		for name, def := range servers {
			srv, err := parseServer(def)
			if err != nil {
				return nil, fmt.Errorf("parse .mcp.json: server %q: %w", name, err)
			}
			f.Servers[name] = srv
		}
	}
	return f, nil
}

func parseServer(def json.RawMessage) (*Server, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(def, &fields); err != nil {
		return nil, fmt.Errorf("definition must be an object: %w", err)
	}

	srv := &Server{}
	for key, raw := range fields {
		var err error
		switch key {
		case "type":
			err = json.Unmarshal(raw, &srv.Type)
		case "command":
			err = json.Unmarshal(raw, &srv.Command)
		case "args":
			err = json.Unmarshal(raw, &srv.Args)
		case "env":
			err = json.Unmarshal(raw, &srv.Env)
		case "url":
			err = json.Unmarshal(raw, &srv.URL)
		case "headers":
			err = json.Unmarshal(raw, &srv.Headers)
		default:
			if _, ok := claudeOnlyFields[key]; ok {
				if srv.ClaudeOnly == nil {
					srv.ClaudeOnly = map[string]json.RawMessage{}
				}
				srv.ClaudeOnly[key] = compactRaw(raw)
			} else {
				if srv.Unknown == nil {
					srv.Unknown = map[string]json.RawMessage{}
				}
				srv.Unknown[key] = compactRaw(raw)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", key, err)
		}
	}

	// 빈 컬렉션은 부재와 의미가 같다. round-trip 시 모델이 일치하도록 nil로 정규화한다.
	if len(srv.Args) == 0 {
		srv.Args = nil
	}
	if len(srv.Env) == 0 {
		srv.Env = nil
	}
	if len(srv.Headers) == 0 {
		srv.Headers = nil
	}
	return srv, nil
}

// Marshal은 내부 모델을 .mcp.json 본문으로 직렬화한다.
// 키는 알파벳 순으로 정렬되고 끝에 개행이 붙는다.
func (f *File) Marshal() ([]byte, error) {
	top := map[string]any{}
	for key, raw := range f.Extra {
		top[key] = raw
	}
	servers := map[string]any{}
	for name, srv := range f.Servers {
		servers[name] = srv.toJSONMap()
	}
	top["mcpServers"] = servers

	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("generate .mcp.json: %w", err)
	}
	return append(out, '\n'), nil
}

// compactRaw는 보존용 raw 값의 whitespace를 제거한다.
// 직렬화 시 들여쓰기가 다시 적용돼도 round-trip 후 모델이 byte 단위로 일치하도록 정규화하는 목적이다.
func compactRaw(raw json.RawMessage) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		// raw는 json.Unmarshal을 통과한 유효한 JSON이므로 실패하지 않는다.
		return raw
	}
	return buf.Bytes()
}

func (s *Server) toJSONMap() map[string]any {
	m := map[string]any{}
	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Command != "" {
		m["command"] = s.Command
	}
	if len(s.Args) > 0 {
		m["args"] = s.Args
	}
	if len(s.Env) > 0 {
		m["env"] = s.Env
	}
	if s.URL != "" {
		m["url"] = s.URL
	}
	if len(s.Headers) > 0 {
		m["headers"] = s.Headers
	}
	for key, raw := range s.ClaudeOnly {
		m[key] = raw
	}
	for key, raw := range s.Unknown {
		m[key] = raw
	}
	return m
}
