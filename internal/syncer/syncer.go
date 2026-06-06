// Package syncer는 project root의 .mcp.json과 .codex/config.toml 사이에서
// MCP 서버 정의를 동기화하는 sync 코어다.
package syncer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/seungyeop-lee/project-mcp-sync/internal/codextoml"
	"github.com/seungyeop-lee/project-mcp-sync/internal/convert"
	"github.com/seungyeop-lee/project-mcp-sync/internal/mcpjson"
)

// Result는 sync 한 번의 결과다. Warnings는 skip된 서버의 사유 목록이다.
type Result struct {
	Changed  bool
	Warnings []string
}

// Run은 root의 .mcp.json과 .codex/config.toml을 동기화한다.
// .mcp.json이 있으면 그것이 source of truth이고, 없으면 .codex/config.toml에서
// .mcp.json을 생성한다. 둘 다 없으면 에러다.
// dryRun이면 파일을 쓰지 않고 결과만 계산한다.
func Run(root string, dryRun bool) (*Result, error) {
	mcpPath := filepath.Join(root, ".mcp.json")
	codexPath := filepath.Join(root, ".codex", "config.toml")

	mcpData, mcpExists, err := readIfExists(mcpPath)
	if err != nil {
		return nil, err
	}
	codexData, codexExists, err := readIfExists(codexPath)
	if err != nil {
		return nil, err
	}

	switch {
	case mcpExists:
		return syncToCodex(mcpData, codexData, codexPath, dryRun)
	case codexExists:
		return syncToMCPJSON(codexData, mcpPath, dryRun)
	default:
		return nil, fmt.Errorf("neither .mcp.json nor .codex/config.toml exists under %s", root)
	}
}

// syncToCodex는 .mcp.json을 source로 .codex/config.toml의 MCP section을 갱신한다.
func syncToCodex(mcpData, codexData []byte, codexPath string, dryRun bool) (*Result, error) {
	mcpFile, err := mcpjson.Parse(mcpData)
	if err != nil {
		return nil, err
	}
	doc, err := codextoml.Parse(codexData)
	if err != nil {
		return nil, err
	}

	res := &Result{}
	for _, name := range mcpFile.Names() {
		set, reason := convert.ToCodex(mcpFile.Servers[name])
		if reason != "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("server %q skipped: %s", name, reason))
			continue
		}
		if err := doc.Upsert(name, set, convert.RemoveList(set)); err != nil {
			return nil, err
		}
	}
	// source에서 사라진 서버는 테이블 통째 삭제. skip한 서버는 source에 남아 있으므로
	// 여기 걸리지 않는다 (동명 기존 테이블 보존).
	for _, name := range doc.Names() {
		if _, ok := mcpFile.Servers[name]; !ok {
			if err := doc.Delete(name); err != nil {
				return nil, err
			}
		}
	}

	out := doc.Bytes()
	res.Changed = !bytes.Equal(out, codexData)
	if res.Changed && !dryRun {
		if err := writeFile(codexPath, out); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// syncToMCPJSON은 .mcp.json이 없을 때 .codex/config.toml에서 .mcp.json을 생성한다.
// codex 파일은 수정하지 않는다.
func syncToMCPJSON(codexData []byte, mcpPath string, dryRun bool) (*Result, error) {
	doc, err := codextoml.Parse(codexData)
	if err != nil {
		return nil, err
	}

	mcpFile := &mcpjson.File{
		Servers: map[string]*mcpjson.Server{},
		Extra:   map[string]json.RawMessage{},
	}
	res := &Result{Changed: true}
	for _, name := range doc.Names() {
		srv, reason := convert.ToMCPJSON(doc.Server(name))
		if reason != "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("server %q skipped: %s", name, reason))
			continue
		}
		mcpFile.Servers[name] = srv
	}

	out, err := mcpFile.Marshal()
	if err != nil {
		return nil, err
	}
	if !dryRun {
		if err := writeFile(mcpPath, out); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func readIfExists(path string) (data []byte, exists bool, err error) {
	data, err = os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
