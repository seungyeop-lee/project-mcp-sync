// Package mcpjson은 Claude Code의 .mcp.json을 내부 모델로 파싱하고,
// 내부 모델에서 .mcp.json을 생성한다.
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

// Claude Code에서만 의미 있는 서버 필드. 파싱은 하되 Codex로 변환할 수 없으므로
// ClaudeOnly에 따로 모아 sync 코어의 skip 판정에 사용한다.
var claudeOnlyFields = map[string]bool{
	"headersHelper": true,
	"oauth":         true,
	"alwaysLoad":    true,
	"timeout":       true,
}

type File struct {
	Servers map[string]*Server
	// mcpServers 외의 최상위 필드. round-trip 시 그대로 보존한다.
	Extra map[string]json.RawMessage
}

type Server struct {
	// Type은 파일에 적힌 값 그대로다. 비어 있으면 EffectiveType이 stdio를 돌려준다.
	Type    string
	Command string
	Args    []string
	Env     map[string]string
	URL     string
	Headers map[string]string
	// ClaudeOnly는 인식된 Claude-only 필드(headersHelper, oauth, alwaysLoad, timeout)다.
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

// Names는 서버 이름을 정렬해 돌려준다. map 순회 순서에 의존하지 않기 위한 helper.
func (f *File) Names() []string {
	names := make([]string, 0, len(f.Servers))
	for name := range f.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
			if claudeOnlyFields[key] {
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

// Marshal은 내부 모델을 .mcp.json 본문으로 직렬화한다. 키는 알파벳 순으로 정렬되고
// 끝에 개행이 붙는다.
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

// compactRaw는 보존용 raw 값의 whitespace를 제거한다. 직렬화 시 들여쓰기가 다시 적용돼도
// round-trip 후 모델이 byte 단위로 일치하도록 정규화하는 목적이다.
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
