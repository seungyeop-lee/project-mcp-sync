// Package syncer는 project root의 .mcp.json과 .codex/config.toml 사이에서
// MCP 서버 정의를 동기화하는 sync 코어다.
package syncer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/seungyeop-lee/project-mcp-sync/internal/codextoml"
	"github.com/seungyeop-lee/project-mcp-sync/internal/convert"
	"github.com/seungyeop-lee/project-mcp-sync/internal/mcpjson"
)

// sync가 변경할 수 있는 파일의 project root 기준 상대 경로.
const (
	mcpFileName   = ".mcp.json"
	codexFileName = ".codex/config.toml"
)

// Plan은 sync 한 번이 수행할 변경의 계산 결과다. 파일은 쓰지 않는다.
// sync --dry-run의 요약과 diff command의 unified diff가 모두 이 값에서 나온다.
type Plan struct {
	root string
	// File은 변경 대상 파일의 root 기준 상대 경로다 (.codex/config.toml 또는 .mcp.json).
	File string
	// Old는 현재 파일 내용이다. 파일이 없으면 nil이다.
	Old []byte
	// New는 sync 후 파일 내용이다.
	New []byte
	// Adds/Updates/Deletes는 대상 파일에서 추가/갱신/삭제될 서버 이름이다 (정렬됨).
	Adds    []string
	Updates []string
	Deletes []string
	// Warnings는 skip된 서버의 사유 목록이다.
	Warnings []string
}

// Run은 root의 .mcp.json과 .codex/config.toml을 동기화한다.
// dryRun이면 파일을 쓰지 않고 계산 결과만 돌려준다.
func Run(root string, dryRun bool) (*Plan, error) {
	plan, err := Compute(root)
	if err != nil {
		return nil, err
	}
	if !dryRun {
		if err := plan.Apply(); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

// Compute는 파일을 쓰지 않고 sync 계획만 계산한다.
// .mcp.json이 있으면 그것이 source of truth이고, 없으면 .codex/config.toml에서
// .mcp.json을 생성한다. 둘 다 없으면 에러다.
func Compute(root string) (*Plan, error) {
	mcpData, mcpExists, err := readIfExists(filepath.Join(root, mcpFileName))
	if err != nil {
		return nil, err
	}
	codexData, codexExists, err := readIfExists(filepath.Join(root, filepath.FromSlash(codexFileName)))
	if err != nil {
		return nil, err
	}

	switch {
	case mcpExists:
		return planCodexUpdate(root, mcpData, codexData)
	case codexExists:
		return planMCPJSONCreate(root, codexData)
	default:
		return nil, fmt.Errorf("neither %s nor %s exists under %s", mcpFileName, codexFileName, root)
	}
}

func (p *Plan) Changed() bool {
	return !bytes.Equal(p.Old, p.New)
}

// Apply는 계획된 변경을 파일에 기록한다. 변경이 없으면 아무것도 하지 않는다.
func (p *Plan) Apply() error {
	if !p.Changed() {
		return nil
	}
	return writeFile(filepath.Join(p.root, filepath.FromSlash(p.File)), p.New)
}

// planCodexUpdate는 .mcp.json을 source로 .codex/config.toml의 MCP section을
// 갱신하는 계획을 계산한다.
func planCodexUpdate(root string, mcpData, codexData []byte) (*Plan, error) {
	mcpFile, err := mcpjson.Parse(mcpData)
	if err != nil {
		return nil, err
	}
	// before는 add/update 분류용 원본 스냅샷, doc은 patch가 적용되는 사본이다.
	before, err := codextoml.Parse(codexData)
	if err != nil {
		return nil, err
	}
	doc, err := codextoml.Parse(codexData)
	if err != nil {
		return nil, err
	}

	plan := &Plan{root: root, File: codexFileName, Old: codexData}
	skipped := map[string]bool{}
	for _, name := range mcpFile.Names() {
		set, reason := convert.ToCodex(mcpFile.Servers[name])
		if reason != "" {
			skipped[name] = true
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("server %q skipped: %s", name, reason))
			continue
		}
		if err := doc.Upsert(name, set, convert.RemoveList(set)); err != nil {
			return nil, err
		}
	}
	// source에서 사라진 서버는 테이블 통째 삭제. skip한 서버는 source에 남아 있으므로
	// 여기 걸리지 않는다 (동명 기존 테이블 보존).
	for _, name := range before.Names() {
		if _, ok := mcpFile.Servers[name]; !ok {
			if err := doc.Delete(name); err != nil {
				return nil, err
			}
			plan.Deletes = append(plan.Deletes, name)
		}
	}

	for _, name := range mcpFile.Names() {
		if skipped[name] {
			continue
		}
		switch {
		case before.Server(name) == nil:
			plan.Adds = append(plan.Adds, name)
		case !reflect.DeepEqual(before.Server(name), doc.Server(name)):
			plan.Updates = append(plan.Updates, name)
		}
	}

	plan.New = doc.Bytes()
	return plan, nil
}

// planMCPJSONCreate는 .mcp.json이 없을 때 .codex/config.toml에서 .mcp.json을
// 생성하는 계획을 계산한다. codex 파일은 수정하지 않는다.
func planMCPJSONCreate(root string, codexData []byte) (*Plan, error) {
	doc, err := codextoml.Parse(codexData)
	if err != nil {
		return nil, err
	}

	mcpFile := &mcpjson.File{
		Servers: map[string]*mcpjson.Server{},
		Extra:   map[string]json.RawMessage{},
	}
	plan := &Plan{root: root, File: mcpFileName}
	for _, name := range doc.Names() {
		srv, reason := convert.ToMCPJSON(doc.Server(name))
		if reason != "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("server %q skipped: %s", name, reason))
			continue
		}
		mcpFile.Servers[name] = srv
		plan.Adds = append(plan.Adds, name)
	}

	plan.New, err = mcpFile.Marshal()
	if err != nil {
		return nil, err
	}
	return plan, nil
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
