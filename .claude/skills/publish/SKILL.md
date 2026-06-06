---
name: publish
description: project-mcp-sync 새 버전을 release한다. 최신 semver tag에서 다음 버전을 계산해 tag 생성·push 후 homebrew-tap formula(url/sha256)를 갱신하고 commit·push까지 진행한다. patch|minor|major 인자가 필요하다.
argument-hint: patch|minor|major
disable-model-invocation: true
---

# publish

project-mcp-sync의 release 절차를 진행한다. 이 스킬의 호출 자체가 push 승인이므로
아래 단계의 git push는 사용자에게 재확인 없이 실행한다.

인자: `$ARGUMENTS` (patch | minor | major 중 하나)

## 0. 인자 검증

`$ARGUMENTS`가 `patch`, `minor`, `major` 중 하나가 아니면 **아무 변경 없이 중단**하고
어떤 bump를 원하는지 질문한다.

## 1. 전제 조건 확인

모두 project root(이 저장소 루트) 기준이다. 하나라도 실패하면 tag를 만들지 않고
중단한 뒤 실패 내용을 보고한다.

- 현재 branch가 `main`이어야 한다.
- `git status --porcelain` 결과가 비어 있어야 한다 (dirty tree면 tag에 누락되는 변경이 생긴다).
- `go test ./...` 가 통과해야 한다.
- tap 저장소가 project root의 sibling `../homebrew-tap`에 있고, 그 안에
  `project-mcp-sync.rb`가 있어야 한다. 없으면 중단하고 tap 위치를 질문한다.
- tap 저장소의 working tree도 clean이어야 한다 (관련 없는 변경이 release commit에 섞이는 것을 막는다).

## 2. 다음 버전 계산

```sh
git tag --list 'v*' --sort=-v:refname | head -1
```

- 최신 tag `vX.Y.Z`를 파싱해 인자에 따라 bump한다:
  - `patch` → `vX.Y.(Z+1)`
  - `minor` → `vX.(Y+1).0`
  - `major` → `v(X+1).0.0`
- tag가 하나도 없으면 중단하고 시작 버전을 질문한다.
- 계산한 버전은 진행 로그로 사용자에게 알린다 (예: `v0.1.0 → v0.1.1`).

## 3. tag 생성·push

```sh
git tag v<NEW>
git push origin main v<NEW>
```

push가 실패하면 로컬 tag를 삭제(`git tag -d v<NEW>`)하고 중단한다.

## 4. tarball sha256 계산

```sh
curl -fsSL -o <tmpfile> https://github.com/seungyeop-lee/project-mcp-sync/archive/refs/tags/v<NEW>.tar.gz
shasum -a 256 <tmpfile>
```

- GitHub의 archive 생성이 push 직후에는 늦을 수 있다. `curl -f`가 실패하면 몇 초
  간격으로 최대 5회 재시도한다.
- 받은 파일이 정상 tar.gz인지 `tar -tzf`로 확인한 뒤 sha256을 계산한다.

## 5. formula 갱신

`../homebrew-tap/project-mcp-sync.rb`에서 두 줄만 수정한다. version 필드는 따로
없고 url에서 자동 추출되므로 다른 줄은 건드리지 않는다.

```ruby
url "https://github.com/seungyeop-lee/project-mcp-sync/archive/refs/tags/v<NEW>.tar.gz"
sha256 "<4단계에서 계산한 값>"
```

## 6. tap commit·push

```sh
git -C ../homebrew-tap add project-mcp-sync.rb
git -C ../homebrew-tap commit -m 'Brew formula update for project-mcp-sync version v<NEW>'
git -C ../homebrew-tap push origin main
```

## 7. 완료 보고

- 새 버전, tarball sha256, project/tap 양쪽 commit·tag를 요약한다.
- 설치 머신에서의 업그레이드 명령을 안내한다: `brew update && brew upgrade project-mcp-sync`
