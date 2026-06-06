// Package envref는 설정 값 안의 ${VAR} 참조를 안전 변환 매트릭스 기준으로 분류한다.
// 분류 결과는 sync 코어의 변환/skip 판정에 사용된다.
package envref

import (
	"regexp"
	"strings"
)

type Kind int

const (
	// None: 값에 ${...} 참조가 없다.
	None Kind = iota
	// FullVar: 값 전체가 ${VAR} 하나다.
	FullVar
	// BearerToken: 값이 정확히 "Bearer ${VAR}" 형태다.
	BearerToken
	// Unsupported: ${...}를 포함하지만 안전 변환 매트릭스 밖의 패턴이다.
	// (${VAR:-default}, 문자열 중간 삽입, 여러 변수 조합 등)
	Unsupported
)

func (k Kind) String() string {
	switch k {
	case None:
		return "none"
	case FullVar:
		return "full-var"
	case BearerToken:
		return "bearer-token"
	case Unsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// Ref는 값 하나의 분류 결과다. Var는 FullVar, BearerToken일 때만 채워진다.
type Ref struct {
	Kind Kind
	Var  string
}

// 환경변수 이름 형태. POSIX 식별자만 안전 변환 대상으로 본다.
const namePattern = `[A-Za-z_][A-Za-z0-9_]*`

var (
	fullVarRe = regexp.MustCompile(`^\$\{(` + namePattern + `)\}$`)
	nameRe    = regexp.MustCompile(`^` + namePattern + `$`)
)

func Classify(value string) Ref {
	if !strings.Contains(value, "${") {
		return Ref{Kind: None}
	}
	if m := fullVarRe.FindStringSubmatch(value); m != nil {
		return Ref{Kind: FullVar, Var: m[1]}
	}
	if rest, ok := strings.CutPrefix(value, "Bearer "); ok {
		if m := fullVarRe.FindStringSubmatch(rest); m != nil {
			return Ref{Kind: BearerToken, Var: m[1]}
		}
	}
	return Ref{Kind: Unsupported}
}

// ValidName은 name이 환경변수 이름 형태인지 검사한다.
func ValidName(name string) bool {
	return nameRe.MatchString(name)
}
