# Behavior Policy

English | [한국어](behavior.ko.md)

This document lays out the complete set of rules for how project-mcp-sync reads and writes files. The goal is that reading this document alone is enough to predict any sync/diff result.

## Project root resolution

1. If `--project <dir>` is given, that directory is used as-is.
2. Otherwise, walk up from the current directory toward its parents and use the nearest directory containing `.git` (a directory, or a worktree's `.git` file) as the project root.
3. If no `.git` is found all the way up to the filesystem root, the absolute path of the current directory is used as the project root.

Even if the wrong directory is picked, the source-of-truth rules below make it an error when both files are missing, so files are never created in an unintended location.

The target files are only `.mcp.json` and `.codex/config.toml` directly under the project root. `.codex/config.toml` files in subdirectories are not searched.

## Source-of-truth detection

| `.mcp.json` | `.codex/config.toml` | behavior |
|---|---|---|
| exists | exists/missing | `.mcp.json` is the source. `.codex/config.toml` is updated (created if missing) |
| missing | exists | `.codex/config.toml` is the source. `.mcp.json` is generated. The codex file is not modified |
| missing | missing | error (exit 2) |

`.mcp.json` is the source of truth **as long as it exists**. This holds even when its content is empty, has no `mcpServers`, or is `{}`; in that case all `[mcp_servers.*]` tables in `.codex/config.toml` are deleted.

### Forcing the source with --source

`sync` and `diff` accept `--source {mcp-json|codex}` to force the source of truth instead of auto-detection.

- The forced source file must exist; otherwise it is an error (exit 2). Any other value for `--source` is also an error (exit 2).
- `--source mcp-json` behaves like the first row of the table above (`.codex/config.toml` is updated, or created if missing).
- `--source codex` with no `.mcp.json` behaves like the generation row. When `.mcp.json` already exists, it is **updated** following the ".codex/config.toml → .mcp.json sync" rules below — this update path is only reachable through `--source codex`.

## .mcp.json → .codex/config.toml sync

### Field-level merge

The codex table fields owned by sync are these 8:

```
command, args, env, env_vars, url, bearer_token_env_var, http_headers, env_http_headers
```

Per server, the behavior is:

- **Add**: servers present only in `.mcp.json` are appended at the end of the file as `[mcp_servers.<name>]` tables.
- **Update**: when a table with the same name exists, only the 8 fields above are overwritten from `.mcp.json`. Managed fields with no corresponding value in the source are removed from the table.
- **Delete**: servers removed from `.mcp.json` have their table deleted entirely (including sub-tables and attached comments).
- **Preserve**: fields other than the 8 above (`enabled`, `required`, `startup_timeout_sec/ms`, `tool_timeout_sec/ms`, `enabled_tools`, `disabled_tools`, `cwd`, and other Codex-only fields) remain as-is for servers with the same name.

### File preservation

Everything outside the `[mcp_servers.*]` blocks is preserved byte-for-byte. Non-MCP settings, comments, key order, and whitespace stay intact. Within an updated table, preserved fields and comments are kept as well.

### Claude-only / unknown fields

- Servers with `headersHelper` or `oauth` are **skipped** because Codex has no corresponding concept.
- Other Claude-only fields such as `alwaysLoad` and `timeout`, and unknown fields, are not a skip reason. The server is converted, and those fields simply remain only in `.mcp.json`.

## .codex/config.toml → .mcp.json generation

Happens when `.mcp.json` is missing (by auto-detection, or with `--source codex`). Each codex server is converted to build a new `.mcp.json` (keys sorted alphabetically).

- Server with `command` → stdio server (`type` omitted)
- Server with `url` → `"type": "http"` server
- Codex-only fields such as `enabled` are not carried over to `.mcp.json`.

## .codex/config.toml → .mcp.json sync

Happens only with `--source codex` when `.mcp.json` already exists. This is the mirror of the `.mcp.json` → codex sync. Per server, the behavior is:

- **Add**: servers present only in codex are added to `mcpServers`.
- **Update**: when a server with the same name exists, the core fields (`type, command, args, env, url, headers`) are replaced with the converted result. Claude-only fields on the existing server follow the classification below; unknown fields are preserved.
- **Delete**: servers removed from codex have their definition deleted entirely, including preserved Claude-only fields.
- Codex-only fields such as `enabled` are not carried over, same as in generation.

### Claude-only field classification on update

| fields | merge behavior | reason |
|---|---|---|
| `alwaysLoad`, `timeout` | preserved | metadata orthogonal to the core fields; the source cannot express it |
| `oauth`, `headersHelper` | removed | alternate means covering the same area (auth/header composition) as the core fields; keeping them next to the overwritten core fields would produce conflicting configuration |
| unknown fields | preserved | preserving is the safe default; if a future field turns out to overlap, it gets added to the classification |

### File rewriting

`.mcp.json` has no comments, so an update reserializes the whole file (keys sorted alphabetically). When the merge result is semantically identical to the current file, the file is left untouched byte-for-byte even if its formatting differs from the serializer's output.

## Skip rules

Servers that cannot be converted are skipped with a warning. **An existing definition on the other side with the same name as a skipped server is left untouched** — the skipped server stays in the source, so it never matches the deletion criteria.

### Skip conditions for the .mcp.json → codex direction

- Servers whose `type` is not `stdio`/`http` (`ws`, `sse`, etc.; an omitted `type` is treated as stdio)
- Use of `headersHelper` or `oauth`
- `${...}` references in `command`, `args`, or `url` values (Codex has no corresponding mechanism)
- `${...}` patterns outside the conversion matrix in `env`/`headers` values (see the matrix below)

### Skip conditions for the codex → .mcp.json direction

- Servers that have both `command` and `url`, or neither
- `${...}` in `command`, `args`, `url`, `env` values, or `http_headers` values — in codex these are literals, but in `.mcp.json` Claude expands them as environment variables, so the meaning would change
- A stdio server with `bearer_token_env_var` or `env_http_headers` (HTTP-only fields)
- A url server with `env_vars` (a stdio-only field)
- An env-reference field whose value is not shaped like an environment variable name (`[A-Za-z_][A-Za-z0-9_]*`)
- Restoration colliding with existing keys: an `env_vars` entry colliding with an `env` key, an `env_http_headers` key colliding with `http_headers`, or `bearer_token_env_var` present while an `Authorization` header is also defined

To prevent producing a `.mcp.json` that silently drops credentials in the codex → mcp.json direction, a server is always skipped when its env-reference fields cannot be restored.

## ${VAR} conversion matrix

Only safe `${VAR}` patterns are converted in both directions. Variable names must be POSIX identifiers (`[A-Za-z_][A-Za-z0-9_]*`).

| .mcp.json | .codex/config.toml | notes |
|---|---|---|
| `"Authorization": "Bearer ${TOKEN}"` in headers | `bearer_token_env_var = "TOKEN"` | only for the `Authorization` header; `Bearer ${VAR}` in any other header is skipped |
| Header value that is entirely `"${VAR}"` | `env_http_headers = { "<header>" = "VAR" }` | |
| Same-name passthrough `"KEY": "${KEY}"` in stdio env | `env_vars = ["KEY"]` | only when the key and variable name match |

Patterns outside the matrix skip that server with a warning:

- Mid-string interpolation (`"https://host/${PATH}"`, `"prefix-${VAR}"`)
- Variant syntax such as `${VAR:-default}`
- Multiple variables combined (`"${A}${B}"`)
- Any `${...}` reference inside `command`/`args`/`url`
- An env value referencing a variable different from its key (`"KEY": "${OTHER}"`)

## enable/disable is not mapped

Claude's `disabledMcpjsonServers` and Codex's `enabled = false` are not mapped to each other. Sync covers server **definitions** only; the enabled state is managed separately in each tool. The `enabled` field in a Codex table remains as a preserved field.

## Command behavior and output

| command | writes files | stdout | stderr |
|---|---|---|---|
| `sync` | only when there are changes | — | skip warnings |
| `sync --dry-run` | never | change summary (add/update/delete) + warnings | — |
| `diff` | never | unified diff when there is drift | skip warnings |

- `sync` does not rewrite a file when the computed result equals the current file.
- The unified diff from `diff` follows git conventions (`a/<file>`, `b/<file>`, `/dev/null` for file creation).

## Exit codes

| command | exit codes |
|---|---|
| `sync`, `sync --dry-run` | 0 = success · 2 = error |
| `diff` | 0 = no drift · 1 = drift · 2 = error |

Skip warnings do not affect the exit code. Use `diff` to check for drift in CI or pre-commit. `sync --dry-run` is a human-readable preview, so it exits 0 even when there is drift.
