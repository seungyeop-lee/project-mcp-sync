# project-mcp-sync

[English](README.md) | 한국어

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
- `--source {mcp-json|codex}`는 자동 판별 대신 source of truth를 강제한다. `--source codex`이면 기존 `.mcp.json`을 codex 기준으로 갱신한다: 코어 필드는 덮어쓰고, 코어 필드와 직교하는 Claude-only 필드(`alwaysLoad`, `timeout`)와 알 수 없는 필드는 보존하며, 겹치는 필드(`oauth`, `headersHelper`)는 제거한다.
- 변환할 수 없는 서버(type이 `ws`/`sse`, `headersHelper`/`oauth` 사용, 변환 매트릭스 밖의 `${VAR}` 패턴)는 skip하고 warning을 출력한다. skip된 서버와 동명인 기존 Codex 테이블은 건드리지 않는다.

source of truth 판별, 필드 단위 merge, `${VAR}` 변환 매트릭스, skip 규칙의 상세 정책은 [docs/behavior.ko.md](docs/behavior.ko.md)를 참조한다.

## 사용법

```sh
project-mcp-sync sync                  # 동기화
project-mcp-sync sync --dry-run        # 변경 요약만 출력, 파일은 쓰지 않음
project-mcp-sync sync --source codex   # codex를 source of truth로 강제 (diff에서도 사용 가능)
project-mcp-sync diff                  # drift 검사, unified diff 출력 (파일을 쓰지 않음)
project-mcp-sync completion zsh        # shell completion (zsh/bash/fish)
project-mcp-sync --version             # 현재 버전 출력
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
