package codextoml

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var bareKeyRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func encodeKey(key string) string {
	if bareKeyRe.MatchString(key) {
		return key
	}
	return quoteString(key)
}

// quoteString은 TOML basic string으로 인코딩한다.
func quoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func encodeValue(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return quoteString(t), nil
	case bool:
		return strconv.FormatBool(t), nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case float64:
		// TOML float은 소수점이 필수다. 정수 값이면 ".0"을 붙여 float으로 유지한다.
		if t == math.Trunc(t) && !math.IsInf(t, 0) && !math.IsNaN(t) {
			return strconv.FormatFloat(t, 'f', 1, 64), nil
		}
		return strconv.FormatFloat(t, 'g', -1, 64), nil
	case []string:
		parts := make([]string, len(t))
		for i, s := range t {
			parts[i] = quoteString(s)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case map[string]string:
		if len(t) == 0 {
			return "{}", nil
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = encodeKey(k) + " = " + quoteString(t[k])
		}
		return "{ " + strings.Join(parts, ", ") + " }", nil
	default:
		return "", fmt.Errorf("unsupported TOML value type %T", v)
	}
}

func renderKeyValue(key string, v any) ([]byte, error) {
	val, err := encodeValue(v)
	if err != nil {
		return nil, fmt.Errorf("field %q: %w", key, err)
	}
	return []byte(encodeKey(key) + " = " + val + "\n"), nil
}

// 새 key 삽입 시 사용하는 관례적 필드 순서. 목록 밖 필드는 알파벳 순으로 뒤에 붙는다.
var fieldOrder = map[string]int{
	"command":              0,
	"args":                 1,
	"env":                  2,
	"env_vars":             3,
	"cwd":                  4,
	"url":                  5,
	"bearer_token_env_var": 6,
	"http_headers":         7,
	"env_http_headers":     8,
}

func sortFields(fields []string) {
	sort.Slice(fields, func(i, j int) bool {
		oi, iok := fieldOrder[fields[i]]
		oj, jok := fieldOrder[fields[j]]
		if iok && jok {
			return oi < oj
		}
		if iok != jok {
			return iok
		}
		return fields[i] < fields[j]
	})
}
