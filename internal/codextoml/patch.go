package codextoml

import (
	"bytes"
	"fmt"
	"sort"
)

type edit struct {
	sp   span
	text []byte // 비어 있으면 삭제
}

func (d *Document) upsertServer(name string, set map[string]any, remove []string) error {
	blocks, err := scanBlocks(d.raw)
	if err != nil {
		return fmt.Errorf("patch config.toml: %w", err)
	}
	main, subs := serverBlocks(blocks, name)

	managed := map[string]bool{}
	for field := range set {
		managed[field] = true
	}
	removeSet := map[string]bool{}
	for _, field := range remove {
		removeSet[field] = true
		managed[field] = true
	}

	var edits []edit
	// 관리 대상 필드가 [mcp_servers.<name>.<field>] sub-table로 존재하면 통째로 제거.
	// 갱신 값은 main 테이블에 inline table로 들어간다.
	for _, sb := range subs {
		if managed[sb.key[2]] {
			edits = append(edits, edit{sp: sb.deleteSpan(d.raw)})
		}
	}

	if main == nil {
		text, err := renderTable(name, set)
		if err != nil {
			return fmt.Errorf("patch config.toml: server %q: %w", name, err)
		}
		eof := span{start: len(d.raw), end: len(d.raw)}
		edits = append(edits, edit{sp: eof, text: append(tableSeparator(d.raw), text...)})
	} else {
		replaced := map[string]bool{}
		for _, it := range main.items {
			if it.kind != itemKV || len(it.key) == 0 {
				continue
			}
			field := it.key[0]
			switch {
			case removeSet[field]:
				edits = append(edits, edit{sp: it.sp})
			case !managed[field]:
				// set/remove에 없는 필드는 그대로 둔다
			case replaced[field]:
				// 같은 필드의 두 번째 이후 표기(dotted key 등)는 제거
				edits = append(edits, edit{sp: it.sp})
			default:
				line, err := renderKeyValue(field, set[field])
				if err != nil {
					return fmt.Errorf("patch config.toml: server %q: %w", name, err)
				}
				edits = append(edits, edit{sp: it.sp, text: line})
				replaced[field] = true
			}
		}

		newFields := make([]string, 0, len(set))
		for field := range set {
			if !replaced[field] {
				newFields = append(newFields, field)
			}
		}
		if len(newFields) > 0 {
			sortFields(newFields)
			var buf bytes.Buffer
			for _, field := range newFields {
				line, err := renderKeyValue(field, set[field])
				if err != nil {
					return fmt.Errorf("patch config.toml: server %q: %w", name, err)
				}
				buf.Write(line)
			}
			pos := main.insertPos()
			edits = append(edits, edit{sp: span{start: pos, end: pos}, text: buf.Bytes()})
		}
	}

	return d.applyAndReload(edits)
}

func (d *Document) deleteServer(name string) error {
	blocks, err := scanBlocks(d.raw)
	if err != nil {
		return fmt.Errorf("patch config.toml: %w", err)
	}
	main, subs := serverBlocks(blocks, name)

	var edits []edit
	if main != nil {
		edits = append(edits, edit{sp: main.deleteSpan(d.raw)})
	}
	for _, sb := range subs {
		edits = append(edits, edit{sp: sb.deleteSpan(d.raw)})
	}
	if len(edits) == 0 {
		return nil
	}
	return d.applyAndReload(edits)
}

func serverBlocks(blocks []*tableBlock, name string) (*tableBlock, []*tableBlock) {
	var main *tableBlock
	var subs []*tableBlock
	for _, b := range blocks {
		if len(b.key) < 2 || b.key[0] != "mcp_servers" || b.key[1] != name {
			continue
		}
		if len(b.key) == 2 {
			main = b
		} else {
			subs = append(subs, b)
		}
	}
	return main, subs
}

func renderTable(name string, set map[string]any) ([]byte, error) {
	fields := make([]string, 0, len(set))
	for field := range set {
		fields = append(fields, field)
	}
	sortFields(fields)

	var buf bytes.Buffer
	buf.WriteString("[mcp_servers." + encodeKey(name) + "]\n")
	for _, field := range fields {
		line, err := renderKeyValue(field, set[field])
		if err != nil {
			return nil, err
		}
		buf.Write(line)
	}
	return buf.Bytes(), nil
}

// tableSeparator는 파일 끝에 새 테이블을 붙일 때 앞에 넣을 구분 bytes다.
func tableSeparator(data []byte) []byte {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if bytes.HasSuffix(data, []byte("\n\n")) {
		return nil
	}
	if bytes.HasSuffix(data, []byte("\n")) {
		return []byte("\n")
	}
	return []byte("\n\n")
}

// applyAndReload는 patch 결과를 재파싱해 유효성을 확인한 뒤에만 문서 상태를 교체한다.
// 결과가 유효한 TOML이 아니면 원본을 건드리지 않고 에러를 돌려준다.
func (d *Document) applyAndReload(edits []edit) error {
	newRaw, err := applyEdits(d.raw, edits)
	if err != nil {
		return fmt.Errorf("patch config.toml: %w", err)
	}
	nd, err := parseDocument(newRaw)
	if err != nil {
		return fmt.Errorf("patch config.toml produced invalid output: %w", err)
	}
	*d = *nd
	return nil
}

func applyEdits(data []byte, edits []edit) ([]byte, error) {
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].sp.start != edits[j].sp.start {
			return edits[i].sp.start < edits[j].sp.start
		}
		return edits[i].sp.end < edits[j].sp.end
	})

	var buf bytes.Buffer
	pos := 0
	for _, e := range edits {
		start, end := e.sp.start, e.sp.end
		if start < pos {
			// 삭제 구간끼리는 빈 줄 흡수 때문에 겹칠 수 있으므로 잘라낸다.
			// 내용이 있는 edit이 겹치는 것은 patcher 버그다.
			if len(e.text) > 0 {
				return nil, fmt.Errorf("internal: overlapping edits at offset %d", start)
			}
			start = pos
		}
		if end < start {
			end = start
		}
		buf.Write(data[pos:start])
		buf.Write(e.text)
		if end > pos {
			pos = end
		}
	}
	buf.Write(data[pos:])
	return buf.Bytes(), nil
}
