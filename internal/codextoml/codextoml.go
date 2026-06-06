// Package codextoml은 .codex/config.toml의 [mcp_servers.*] 테이블을 읽고, MCP section만 수정하며 나머지 내용(주석, 비-MCP 설정, 키 순서, 공백)을 byte 단위로 보존하는 patcher를 제공한다.
//
// go-toml v2의 일반 Marshal은 주석을 유실하므로 쓰기 경로에 사용하지 않는다.
// 대신 unstable parser로 각 expression의 byte range를 얻어 [mcp_servers.*] 블록만 잘라내거나 바꿔 넣는다.
// 건드리지 않는 영역은 원본 byte가 그대로 남는다.
package codextoml

import "sort"

type Document struct {
	raw     []byte
	servers map[string]*Server
}

type Server struct {
	Command           string
	Args              []string
	Env               map[string]string
	EnvVars           []string
	URL               string
	BearerTokenEnvVar string
	HTTPHeaders       map[string]string
	EnvHTTPHeaders    map[string]string
	// Other는 sync가 읽지 않는 Codex-only 필드(enabled, required, cwd, timeout류, enabled_tools 등)와 알 수 없는 필드다.
	// 파일 보존은 patcher가 byte 단위로 보장하므로 이 값은 읽기(diff 표시 등) 용도다.
	Other map[string]any
}

func Parse(data []byte) (*Document, error) {
	return parseDocument(data)
}

func (d *Document) Servers() map[string]*Server {
	return d.servers
}

func (d *Document) Server(name string) *Server {
	return d.servers[name]
}

// Names는 서버 이름을 정렬해 돌려준다. map 순회 순서에 의존하지 않기 위한 helper.
func (d *Document) Names() []string {
	names := make([]string, 0, len(d.servers))
	for name := range d.servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (d *Document) Bytes() []byte {
	return append([]byte(nil), d.raw...)
}

// Upsert는 서버 테이블이 있으면 set의 필드만 갱신하고(나머지 필드와 주석 보존), 없으면 파일 끝에 새 테이블을 추가한다.
// remove의 필드는 테이블에서 제거한다.
func (d *Document) Upsert(name string, set map[string]any, remove []string) error {
	return d.upsertServer(name, set, remove)
}

// Delete는 서버의 테이블(sub-table 포함)을 붙은 주석과 함께 통째로 제거한다.
// 서버가 없으면 아무것도 하지 않는다.
func (d *Document) Delete(name string) error {
	return d.deleteServer(name)
}
