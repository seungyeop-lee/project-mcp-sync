// Package version은 버전 상수를 보관한다.
package version

// Version은 현재 release 버전이다. git tag와 같은 v접두사 semver 형식을 유지하며, release 절차(.claude/skills/publish)가 tag 생성 전에 이 값을 새 버전으로 갱신한다.
const Version = "v0.2.0"
