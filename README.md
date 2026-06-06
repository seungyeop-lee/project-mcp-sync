# project-mcp-sync

English | [한국어](README.ko.md)

A CLI that syncs project-scoped MCP server definitions between Claude Code's `.mcp.json` and Codex's `.codex/config.toml`.

## Installation

```sh
brew install seungyeop-lee/tap/project-mcp-sync
```

Shell completions (zsh/bash/fish) are installed into the Homebrew completion paths.

## How it works

- If `.mcp.json` exists, it is the source of truth and the `[mcp_servers.*]` tables in `.codex/config.toml` are updated. Non-MCP settings, comments, and Codex-only fields (`enabled`, timeouts, etc.) are preserved as-is.
- If `.mcp.json` does not exist, `.mcp.json` is generated from `.codex/config.toml`. The codex file is not modified in this case.
- If neither exists, it is an error.
- Servers that cannot be converted (type `ws`/`sse`, use of `headersHelper`/`oauth`, `${VAR}` patterns outside the conversion matrix) are skipped with a warning. Existing Codex tables with the same name as a skipped server are left untouched.

For the detailed policy on source-of-truth detection, field-level merge, the `${VAR}` conversion matrix, and skip rules, see [docs/behavior.md](docs/behavior.md).

## Usage

```sh
project-mcp-sync sync              # synchronize
project-mcp-sync sync --dry-run    # print a change summary only, write no files
project-mcp-sync diff              # check for drift, print a unified diff (writes no files)
project-mcp-sync completion zsh    # shell completion (zsh/bash/fish)
```

The project root is found from the current directory by looking for the nearest `.git`; if there is no `.git`, the current directory is used as-is. You can specify it directly with `--project <dir>`.

## Exit codes

| command | exit codes |
|---|---|
| `sync`, `sync --dry-run` | 0 = success · 2 = error |
| `diff` | 0 = no drift · 1 = drift · 2 = error |

Use the `diff` command to check for drift in CI or a pre-commit hook.
`sync --dry-run` is a human-readable preview, so it exits 0 even when there is drift.

```sh
# pre-commit example: block the commit when there is drift
project-mcp-sync diff || exit 1
```
