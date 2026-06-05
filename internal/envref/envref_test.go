package envref

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		value string
		want  Ref
	}{
		// 변수 없음
		{"plain-value", Ref{Kind: None}},
		{"", Ref{Kind: None}},
		{"Bearer abc123", Ref{Kind: None}},
		{"$HOME", Ref{Kind: None}},

		// 값 전체가 변수
		{"${TOKEN}", Ref{Kind: FullVar, Var: "TOKEN"}},
		{"${_PRIVATE_VAR1}", Ref{Kind: FullVar, Var: "_PRIVATE_VAR1"}},

		// Bearer 접두
		{"Bearer ${GITHUB_PAT}", Ref{Kind: BearerToken, Var: "GITHUB_PAT"}},

		// default 문법은 변환 불가
		{"${VAR:-default}", Ref{Kind: Unsupported}},
		{"Bearer ${VAR:-default}", Ref{Kind: Unsupported}},

		// 문자열 조합은 변환 불가
		{"https://example.com/${TENANT}/mcp", Ref{Kind: Unsupported}},
		{"prefix ${VAR}", Ref{Kind: Unsupported}},
		{"${VAR} suffix", Ref{Kind: Unsupported}},
		{"${A}${B}", Ref{Kind: Unsupported}},

		// Bearer 형태에서 벗어난 변형은 변환 불가
		{"bearer ${TOKEN}", Ref{Kind: Unsupported}},
		{"Bearer  ${TOKEN}", Ref{Kind: Unsupported}},
		{"Bearer ${TOKEN} extra", Ref{Kind: Unsupported}},

		// 식별자가 아닌 변수명, 닫히지 않은 참조는 변환 불가
		{"${1BAD}", Ref{Kind: Unsupported}},
		{"${}", Ref{Kind: Unsupported}},
		{"${UNCLOSED", Ref{Kind: Unsupported}},
	}
	for _, tc := range cases {
		if got := Classify(tc.value); got != tc.want {
			t.Errorf("Classify(%q) = %+v, want %+v", tc.value, got, tc.want)
		}
	}
}
