# dev-pulse

Read-only MCP server, built on the official
[`github.com/modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk),
exposing "workspace pulse" tools over a local git workspace root
(`DEV_PULSE_ROOT`, default `$HOME/development`): repository status, CI run
status, open PR queue, and stale local branches.

The server talks MCP over stdio (`mcp.StdioTransport`); all logging goes to
stderr so stdout stays reserved for the JSON-RPC channel.

## Tools

- **`repo_status(root?, name?)`** — lists git repos under the root; for each,
  the current branch, ahead/behind counts vs upstream, and counts of dirty
  and untracked files. `name` filters by substring.
- **`ci_status(repo, limit=10)`** — recent GitHub Actions runs for `repo`
  (a local repo name under the root, or an `owner/name` slug), via
  `gh run list`. Requires `gh` on `PATH`; returns a tool error if missing.
- **`pr_queue(repo?)`** — open pull requests with review decision and CI
  rollup state, via `gh pr list`. Covers every repo under the root when
  `repo` is omitted.
- **`stale_branches(days=30, repo?)`** — local branches with no commit in
  the last `days` days, across one or all repos under the root.

All four tools are annotated `readOnlyHint: true`, `destructiveHint: false`,
`idempotentHint: true`.

## Architecture

- `cmd/dev-pulse/main.go` — entrypoint, wires `internal/tools` onto an
  `mcp.Server` and runs it over stdio.
- `internal/execx` — a small `Runner` interface wrapping
  `exec.CommandContext` with argv-style arguments (never shell string
  concatenation), so command execution is injectable for offline tests.
- `internal/gitx` — repo discovery under the root and parsing of
  `git status --porcelain=v2 --branch` / `git for-each-ref` output.
- `internal/ghx` — wraps `gh run list` / `gh pr list --json` and parses
  their JSON output.
- `internal/getorigin` — parses a `git remote get-url origin` value (SSH or
  HTTPS GitHub form) into an `owner/repo` slug for `gh -R`.
- `internal/tools` — the four MCP tool handlers, input/output schemas, and
  path/root validation (rejects `..` segments in a caller-supplied root or
  repo name).

Every unit test runs offline against a fake `execx.Runner` or real temp
directories — no network access or `gh` auth required.

## Build

```bash
go build ./...
```

## Run manually

```bash
DEV_PULSE_ROOT=$HOME/development go run ./cmd/dev-pulse
```

The server then reads newline-delimited JSON-RPC from stdin and writes
responses to stdout; logs go to stderr. Example initialize + tools/list
smoke test (executed during development):

```
$ go build -o /tmp/dev-pulse ./cmd/dev-pulse
$ python3 smoke.py   # sends initialize, notifications/initialized, tools/list
INIT RESPONSE: {"jsonrpc":"2.0","id":1,"result":{"capabilities":{"logging":{},"tools":{"listChanged":true}},"protocolVersion":"2025-11-25","serverInfo":{"name":"dev-pulse","version":"v0.1.0"}}}
TOOLS/LIST RESPONSE: {"jsonrpc":"2.0","id":2,"result":{...,"tools":[{"name":"ci_status",...},{"name":"pr_queue",...},{"name":"repo_status",...},{"name":"stale_branches",...}]}}
STDERR: time=2026-10-03T23:27:20.241-04:00 level=INFO msg="dev-pulse starting" transport=stdio
```

The real `tools/list` response returns full JSON Schema input/output
definitions for `ci_status`, `pr_queue`, `repo_status`, and
`stale_branches` (elided above for brevity).

## Verification

Commands actually run against this repo, with real results:

```bash
$ gofmt -l .
(no output — all files formatted)

$ go vet ./...
(no output — clean)

$ go build ./...
(no output — clean)

$ go test -race ./...
ok  	github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/getorigin
ok  	github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/ghx
ok  	github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/gitx
ok  	github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/tools

$ go tool golangci-lint run ./...
0 issues.
```

`golangci-lint` is pinned as a Go tool dependency in `go.mod`
(`go get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`)
— v2.14.0 is required because older v2 releases (e.g. v2.9.0) cannot parse
Go 1.27's export data format.

## CI

`.github/workflows/ci.yml` runs two required jobs on every pull request and
push to `main`:

- **`lint`** — `gofmt -l .` (fails on unformatted files), `go vet ./...`,
  `go tool golangci-lint run`.
- **`test`** — `go build ./...`, `go test -race ./...`.

Both jobs check out with `actions/checkout` and set up Go with
`actions/setup-go` (`go-version-file: go.mod`), both pinned to full commit
SHAs. Workflow permissions are `contents: read`; checkout uses
`persist-credentials: false`. No `gh` auth or network access beyond Go
module downloads is required.
