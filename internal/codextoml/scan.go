package codextoml

import (
	"bytes"

	"github.com/pelletier/go-toml/v2/unstable"
)

// span은 raw bytes의 [start, end) 구간이다.
// 줄 경계까지 확장되어 있어 end는 개행 문자 다음(또는 EOF)을 가리킨다.
type span struct {
	start, end int
}

type itemKind int

const (
	itemKV itemKind = iota
	itemComment
)

type tableItem struct {
	kind itemKind
	key  []string // KV의 dotted key 경로. comment면 nil
	sp   span
}

// tableBlock은 테이블 header와 그 아래 항목들이 차지하는 byte 구간이다.
type tableBlock struct {
	key       []string
	header    span
	leadStart int // header 바로 위에 빈 줄 없이 붙은 연속 주석의 시작 (없으면 header.start)
	items     []tableItem
}

// scanBlocks는 unstable parser로 top-level expression의 byte range를 수집해 테이블 블록 목록으로 묶는다.
// 첫 테이블 이전의 root key-value는 patch 대상이 아니므로 버린다.
func scanBlocks(data []byte) ([]*tableBlock, error) {
	type expr struct {
		kind unstable.Kind
		key  []string
		sp   span
	}

	p := &unstable.Parser{KeepComments: true}
	p.Reset(data)
	var exprs []expr
	for p.NextExpression() {
		node := p.Expression()
		minOff, maxOff := -1, -1
		nodeExtent(node, &minOff, &maxOff)
		if minOff < 0 {
			continue
		}
		e := expr{kind: node.Kind, sp: lineSpan(data, minOff, maxOff)}
		switch node.Kind {
		case unstable.Table, unstable.ArrayTable, unstable.KeyValue:
			it := node.Key()
			for it.Next() {
				e.key = append(e.key, string(it.Node().Data))
			}
		}
		exprs = append(exprs, e)
	}
	if err := p.Error(); err != nil {
		return nil, err
	}

	var blocks []*tableBlock
	var current *tableBlock
	for i, e := range exprs {
		switch e.kind {
		case unstable.Table, unstable.ArrayTable:
			b := &tableBlock{key: e.key, header: e.sp, leadStart: e.sp.start}
			for j := i - 1; j >= 0 && exprs[j].kind == unstable.Comment && exprs[j].sp.end == b.leadStart; j-- {
				b.leadStart = exprs[j].sp.start
			}
			blocks = append(blocks, b)
			current = b
		case unstable.KeyValue:
			if current != nil {
				current.items = append(current.items, tableItem{kind: itemKV, key: e.key, sp: e.sp})
			}
		case unstable.Comment:
			if current != nil {
				current.items = append(current.items, tableItem{kind: itemComment, sp: e.sp})
			}
		}
	}
	return blocks, nil
}

// nodeExtent는 node 서브트리가 가리키는 원본 byte 구간의 최소/최대 offset을 구한다.
// Table/KeyValue node 자체에는 Raw가 없고 자식(key, value)에만 있다.
func nodeExtent(n *unstable.Node, minOff, maxOff *int) {
	if n.Raw.Length > 0 {
		s := int(n.Raw.Offset)
		e := s + int(n.Raw.Length)
		if *minOff < 0 || s < *minOff {
			*minOff = s
		}
		if e > *maxOff {
			*maxOff = e
		}
	}
	for it := n.Children(); it.Next(); {
		nodeExtent(it.Node(), minOff, maxOff)
	}
}

// lineSpan은 [minOff, maxOff) 구간을 줄 경계까지 확장한다.
// TOML은 top-level expression이 한 줄에 하나뿐이므로, 줄 확장으로 테이블 괄호, '=', 같은 줄 주석까지 안전하게 포함할 수 있다.
func lineSpan(data []byte, minOff, maxOff int) span {
	start := bytes.LastIndexByte(data[:minOff], '\n') + 1
	end := len(data)
	if i := bytes.IndexByte(data[maxOff:], '\n'); i >= 0 {
		end = maxOff + i + 1
	}
	return span{start: start, end: end}
}

// deleteSpan은 블록을 통째로 지울 때 제거할 구간이다.
// header에 붙은 선행 주석과 마지막 key 뒤에 붙은 연속 주석을 포함하고, 뒤따르는 빈 줄을 흡수한다.
// 빈 줄로 분리된 주석은 다음 section의 것일 수 있으므로 남긴다.
func (b *tableBlock) deleteSpan(data []byte) span {
	end := b.header.end
	lastKV := -1
	for i, it := range b.items {
		if it.kind == itemKV {
			lastKV = i
		}
	}
	if lastKV >= 0 {
		end = b.items[lastKV].sp.end
	}
	for i := lastKV + 1; i < len(b.items); i++ {
		if b.items[i].kind == itemComment && b.items[i].sp.start == end {
			end = b.items[i].sp.end
		} else {
			break
		}
	}
	end = absorbBlankLines(data, end)

	start := b.leadStart
	// 파일 끝까지 지우는 경우 앞의 빈 줄도 정리해 흔적을 남기지 않는다.
	if end == len(data) {
		start = trimBlankLinesBackward(data, start)
	}
	return span{start: start, end: end}
}

// insertPos는 블록에 새 key-value를 끼워 넣을 위치(마지막 KV 다음 줄)다.
func (b *tableBlock) insertPos() int {
	pos := b.header.end
	for _, it := range b.items {
		if it.kind == itemKV {
			pos = it.sp.end
		}
	}
	return pos
}

func absorbBlankLines(data []byte, pos int) int {
	for pos < len(data) {
		lineEnd := len(data)
		if i := bytes.IndexByte(data[pos:], '\n'); i >= 0 {
			lineEnd = pos + i + 1
		}
		if len(bytes.TrimSpace(data[pos:lineEnd])) != 0 {
			break
		}
		pos = lineEnd
	}
	return pos
}

func trimBlankLinesBackward(data []byte, pos int) int {
	for pos > 0 {
		lineStart := bytes.LastIndexByte(data[:pos-1], '\n') + 1
		if len(bytes.TrimSpace(data[lineStart:pos])) != 0 {
			break
		}
		pos = lineStart
	}
	return pos
}
