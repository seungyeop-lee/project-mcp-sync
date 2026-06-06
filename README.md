# project-mcp-sync

Claude Code의 `.mcp.json`과 Codex의 `.codex/config.toml` 사이에서 project-scoped MCP 서버 정의를 동기화하는 CLI.

## 설치

```sh
brew install seungyeop-lee/tap/project-mcp-sync
```

shell completion(zsh/bash/fish)은 Homebrew completion 경로에 함께 설치된다.

## 동작 방식

- `.mcp.json`이 있으면 source of truth로 삼아 `.codex/config.toml`의 `[mcp_servers.*]` 테이블을 갱신한다. 비-MCP 설정, 주석, Codex-only 필드(`enabled`, timeout류 등)는 그대로 보존된다.
- `.mcp.json`이 없으면 `.codex/config.toml`에서 `.mcp.json`을 생성한다. 이때 codex 파일은 수정하지 않는다.
- 둘 다 없으면 오류.
- 변환할 수 없는 서버(type이 `ws`/`sse`, `headersHelper`/`oauth` 사용, 변환 매트릭스 밖의 `${VAR}` 패턴)는 skip하고 warning을 출력한다. skip된 서버와 동명인 기존 Codex 테이블은 건드리지 않는다.

## 사용법

```sh
project-mcp-sync sync              # 동기화
project-mcp-sync sync --dry-run    # 변경 요약만 출력, 파일은 쓰지 않음
project-mcp-sync diff              # drift 검사, unified diff 출력 (파일을 쓰지 않음)
project-mcp-sync completion zsh    # shell completion (zsh/bash/fish)
```

project root는 현재 디렉토리에서 가장 가까운 `.git` 기준으로 찾고, `.git`이 없으면 현재 디렉토리를 그대로 사용한다. `--project <dir>`로 직접 지정할 수 있다.

## Exit codes

| command | exit codes |
|---|---|
| `sync`, `sync --dry-run` | 0 = 성공 · 2 = 오류 |
| `diff` | 0 = drift 없음 · 1 = drift 있음 · 2 = 오류 |

CI나 pre-commit hook에서 drift를 검사할 때는 `diff` command를 사용한다.
`sync --dry-run`은 사람이 읽는 미리보기 용도라 drift가 있어도 exit 0이다.

```sh
# pre-commit 예시: drift가 있으면 commit을 막는다
project-mcp-sync diff || exit 1
```

## ${VAR} 변환 매트릭스

`.mcp.json`의 안전한 `${VAR}` 패턴은 Codex의 env 참조 필드와 양방향 변환된다.

| .mcp.json | .codex/config.toml |
|---|---|
| headers의 `"Authorization": "Bearer ${TOKEN}"` | `bearer_token_env_var = "TOKEN"` |
| 헤더 값 전체가 `"${VAR}"` | `env_http_headers = { "<헤더>" = "VAR" }` |
| stdio env의 동명 passthrough `"KEY": "${KEY}"` | `env_vars = ["KEY"]` |

매트릭스 밖 패턴(url 중간 삽입, `${VAR:-default}`, 문자열 조합, command/args 안의 `${VAR}`)은 해당 서버를 skip + warning 처리한다.
