# 동작 정책

[English](behavior.md) | 한국어

project-mcp-sync가 파일을 읽고 쓰는 규칙 전체를 정리한 문서다. 이 문서만 읽고 sync/diff 결과를 예측할 수 있는 것을 목표로 한다.

## Project root 결정

1. `--project <dir>`가 지정되면 그 디렉토리를 그대로 사용한다.
2. 지정이 없으면 현재 디렉토리에서 부모로 올라가며 `.git`(디렉토리 또는 worktree의 `.git` 파일)이 있는 가장 가까운 디렉토리를 project root로 삼는다.
3. filesystem root까지 `.git`이 없으면 현재 디렉토리의 절대경로를 project root로 사용한다.

잘못된 디렉토리가 잡혀도 아래 source of truth 규칙에 따라 두 파일 모두 없으면 에러가 나므로, 의도하지 않은 위치에 파일이 생기지는 않는다.

대상 파일은 project root 바로 아래의 `.mcp.json`과 `.codex/config.toml` 둘뿐이다. 하위 디렉토리의 `.codex/config.toml`은 탐색하지 않는다.

## Source of truth 판별

| `.mcp.json` | `.codex/config.toml` | 동작 |
|---|---|---|
| 있음 | 있음/없음 | `.mcp.json`이 source. `.codex/config.toml`을 갱신한다 (없으면 생성) |
| 없음 | 있음 | `.codex/config.toml`이 source. `.mcp.json`을 생성한다. codex 파일은 수정하지 않는다 |
| 없음 | 없음 | 에러 (exit 2) |

`.mcp.json`은 **존재하기만 하면** source of truth다. 내용이 비어 있거나 `mcpServers`가 없거나 `{}`여도 마찬가지이며, 이 경우 `.codex/config.toml`의 `[mcp_servers.*]` 테이블이 전부 삭제된다.

### --source로 source 강제

`sync`와 `diff`는 `--source {mcp-json|codex}`로 자동 판별 대신 source of truth를 강제할 수 있다.

- 강제한 source 파일이 없으면 에러다 (exit 2). `--source`에 그 외의 값을 주는 것도 에러다 (exit 2).
- `--source mcp-json`은 위 표의 첫 행과 같게 동작한다 (`.codex/config.toml`을 갱신하고, 없으면 생성한다).
- `--source codex`는 `.mcp.json`이 없으면 생성 행과 같게 동작한다. `.mcp.json`이 이미 있으면 아래 ".codex/config.toml → .mcp.json 동기화" 규칙에 따라 **갱신**한다 — 이 갱신 경로는 `--source codex`로만 진입할 수 있다.

## .mcp.json → .codex/config.toml 동기화

### 필드 단위 merge

sync가 소유하는 codex 테이블 필드는 다음 8개다:

```
command, args, env, env_vars, url, bearer_token_env_var, http_headers, env_http_headers
```

서버별로 다음과 같이 동작한다:

- **추가**: `.mcp.json`에만 있는 서버는 파일 끝에 `[mcp_servers.<name>]` 테이블로 추가된다.
- **갱신**: 동명 테이블이 있으면 위 8개 필드만 `.mcp.json` 기준으로 덮어쓴다. source에 해당 값이 없는 관리 필드는 테이블에서 제거한다.
- **삭제**: `.mcp.json`에서 사라진 서버는 테이블(sub-table, 붙은 주석 포함)을 통째로 삭제한다.
- **보존**: 위 8개 외의 필드(`enabled`, `required`, `startup_timeout_sec/ms`, `tool_timeout_sec/ms`, `enabled_tools`, `disabled_tools`, `cwd` 등 Codex-only 필드)는 동명 서버에 한해 그대로 남는다.

### 파일 보존

`[mcp_servers.*]` 블록 밖은 byte 단위로 보존된다. 비-MCP 설정, 주석, 키 순서, 공백이 그대로 유지된다. 갱신되는 테이블 안에서도 보존 필드와 주석은 유지된다.

### Claude-only / 알 수 없는 필드

- `headersHelper`, `oauth`가 있는 서버는 Codex에 대응 개념이 없어 **skip**된다.
- `alwaysLoad`, `timeout` 등 그 외 Claude-only 필드와 알 수 없는 필드는 skip 사유가 아니다. 서버는 변환되고, 해당 필드는 `.mcp.json`에만 남는다.

## .codex/config.toml → .mcp.json 생성

`.mcp.json`이 없을 때 일어난다 (자동 판별 또는 `--source codex`). codex의 각 서버를 변환해 새 `.mcp.json`을 만든다 (키는 알파벳 순 정렬).

- `command`가 있는 서버 → stdio 서버 (`type` 생략)
- `url`이 있는 서버 → `"type": "http"` 서버
- `enabled` 등 Codex-only 필드는 `.mcp.json`으로 옮기지 않는다.

## .codex/config.toml → .mcp.json 동기화

`--source codex`이면서 `.mcp.json`이 이미 있을 때만 일어난다. `.mcp.json` → codex 동기화의 거울상이다. 서버별로 다음과 같이 동작한다:

- **추가**: codex에만 있는 서버는 `mcpServers`에 추가된다.
- **갱신**: 동명 서버가 있으면 코어 필드(`type, command, args, env, url, headers`)를 변환 결과로 교체한다. 기존 서버의 Claude-only 필드는 아래 분류를 따르고, 알 수 없는 필드는 보존한다.
- **삭제**: codex에서 사라진 서버는 보존 대상 Claude-only 필드까지 정의를 통째로 삭제한다.
- `enabled` 등 Codex-only 필드는 생성 때와 마찬가지로 옮기지 않는다.

### 갱신 시 Claude-only 필드 분류

| 필드 | merge 동작 | 이유 |
|---|---|---|
| `alwaysLoad`, `timeout` | 보존 | 코어 필드와 직교하는 메타데이터로, source가 표현할 수 없는 정보다 |
| `oauth`, `headersHelper` | 제거 | 코어 필드와 같은 영역(인증·헤더 구성)을 다루는 대체 수단으로, 덮어쓴 코어 필드 옆에 남기면 충돌하는 설정이 만들어진다 |
| 알 수 없는 필드 | 보존 | 보존이 안전한 기본값이다. 이후 겹침류로 판명되는 필드는 분류에 추가한다 |

### 파일 재작성

`.mcp.json`에는 주석이 없으므로 갱신 시 파일 전체를 다시 직렬화한다 (키는 알파벳 순 정렬). merge 결과가 현재 파일과 의미상 같으면 포맷이 직렬화 결과와 달라도 파일을 byte 단위 그대로 둔다.

## Skip 규칙

변환할 수 없는 서버는 건너뛰고 warning을 낸다. **skip된 서버와 동명인 기존 반대편 정의는 건드리지 않는다** — skip 서버는 source에 남아 있으므로 삭제 대상에 걸리지 않는다.

### .mcp.json → codex 방향 skip 조건

- `type`이 `stdio`/`http`가 아닌 서버 (`ws`, `sse` 등. `type` 생략은 stdio로 본다)
- `headersHelper` 또는 `oauth` 사용
- `command`, `args`, `url` 값에 `${...}` 참조 포함 (Codex에 대응 수단이 없다)
- `env`/`headers` 값에 변환 매트릭스 밖의 `${...}` 패턴 포함 (아래 매트릭스 참조)

### codex → .mcp.json 방향 skip 조건

- `command`와 `url`이 둘 다 있거나 둘 다 없는 서버
- `command`, `args`, `url`, `env` 값, `http_headers` 값에 `${...}` 포함 — codex에서는 리터럴이지만 `.mcp.json`에서는 Claude가 환경변수로 확장하므로 의미가 달라진다
- stdio 서버에 `bearer_token_env_var` 또는 `env_http_headers`가 있는 경우 (HTTP 전용 필드)
- url 서버에 `env_vars`가 있는 경우 (stdio 전용 필드)
- env 참조 필드 값이 환경변수 이름 형태(`[A-Za-z_][A-Za-z0-9_]*`)가 아닌 경우
- 복원 결과가 기존 키와 충돌하는 경우: `env_vars` 항목이 `env` 키와 충돌, `env_http_headers` 키가 `http_headers`와 충돌, `bearer_token_env_var`가 있는데 `Authorization` 헤더도 정의됨

codex → mcp.json 방향에서 인증 정보가 조용히 빠진 `.mcp.json`이 생기는 것을 막기 위해, env 참조 필드를 복원할 수 없으면 항상 skip한다.

## ${VAR} 변환 매트릭스

안전한 `${VAR}` 패턴만 양방향 변환한다. 변수 이름은 POSIX 식별자(`[A-Za-z_][A-Za-z0-9_]*`)만 허용한다.

| .mcp.json | .codex/config.toml | 비고 |
|---|---|---|
| headers의 `"Authorization": "Bearer ${TOKEN}"` | `bearer_token_env_var = "TOKEN"` | `Authorization` 헤더에서만. 다른 헤더의 `Bearer ${VAR}`는 skip |
| 헤더 값 전체가 `"${VAR}"` | `env_http_headers = { "<헤더>" = "VAR" }` | |
| stdio env의 동명 passthrough `"KEY": "${KEY}"` | `env_vars = ["KEY"]` | 키와 변수 이름이 같을 때만 |

매트릭스 밖 패턴은 해당 서버 skip + warning:

- 문자열 중간 삽입 (`"https://host/${PATH}"`, `"prefix-${VAR}"`)
- `${VAR:-default}` 등 변형 문법
- 여러 변수 조합 (`"${A}${B}"`)
- `command`/`args`/`url` 안의 모든 `${...}` 참조
- env에서 키와 다른 변수 참조 (`"KEY": "${OTHER}"`)

## enable/disable 비매핑

Claude의 `disabledMcpjsonServers`와 Codex의 `enabled = false`는 서로 매핑하지 않는다. sync 대상은 서버 **정의**뿐이며, 활성화 상태는 각 도구에서 따로 관리한다. Codex 테이블의 `enabled` 필드는 보존 필드로 그대로 남는다.

## Command 동작과 출력

| command | 파일 쓰기 | stdout | stderr |
|---|---|---|---|
| `sync` | 변경이 있을 때만 | — | skip warning |
| `sync --dry-run` | 안 함 | 변경 요약(add/update/delete) + warning | — |
| `diff` | 안 함 | drift가 있으면 unified diff | skip warning |

- `sync`는 계산 결과가 현재 파일과 같으면 파일을 다시 쓰지 않는다.
- `diff`의 unified diff는 git 관례를 따른다 (`a/<file>`, `b/<file>`, 파일 생성은 `/dev/null`).

## Exit code

| command | exit codes |
|---|---|
| `sync`, `sync --dry-run` | 0 = 성공 · 2 = 오류 |
| `diff` | 0 = drift 없음 · 1 = drift 있음 · 2 = 오류 |

skip warning은 exit code에 영향을 주지 않는다. CI나 pre-commit에서 drift를 검사할 때는 `diff`를 사용한다. `sync --dry-run`은 사람이 읽는 미리보기 용도라 drift가 있어도 exit 0이다.
