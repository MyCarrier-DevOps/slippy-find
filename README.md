# slippy-find

A Go CLI application that resolves routing slips from local Git repository commit history, outputting only the `correlation_id` for consumption by external systems.

## Overview

`slippy-find` walks the commit ancestry of a local Git repository and asks the **slippy-api** service for the most recent routing slip associated with any commit in the history. On success it prints the matching `correlation_id` to stdout for pipeline orchestration.

### Key Features

- **Local Git operations only** — no GitHub API calls; works entirely with local repositories
- **Commit ancestry walking** — uses `go-git/v5` to traverse commit history from HEAD
- **slippy-api HTTP client** — looks up slips via `POST /slips/find-by-commits` using the [`slippy-api/slippy-client`](https://github.com/MyCarrier-DevOps/slippy-api) generated client (bearer-token auth, 30s timeout)
- **Clean architecture** — full dependency injection for testability

## Installation

### Using `go install`

```bash
go install github.com/MyCarrier-DevOps/slippy-find@latest
```

### GitHub Actions

Use the provided action to install pre-built binaries (fastest):

```yaml
- name: Install slippy-find
  uses: MyCarrier-DevOps/slippy-find/.github/actions/setup-slippy-find@main

- name: Run slippy-find
  env:
    SLIPPY_API_URL: ${{ vars.SLIPPY_API_URL }}
    SLIPPY_API_KEY: ${{ secrets.SLIPPY_API_KEY }}
  run: slippy-find
```

To pin to a specific version:

```yaml
- uses: MyCarrier-DevOps/slippy-find/.github/actions/setup-slippy-find@main
  with:
    version: v0.2.0
```

### Download Binary

Download pre-built binaries from [GitHub Releases](https://github.com/MyCarrier-DevOps/slippy-find/releases):

```bash
# Linux (amd64)
curl -sL https://github.com/MyCarrier-DevOps/slippy-find/releases/latest/download/slippy-find-linux-amd64 -o slippy-find
chmod +x slippy-find
sudo mv slippy-find /usr/local/bin/

# macOS (Apple Silicon)
curl -sL https://github.com/MyCarrier-DevOps/slippy-find/releases/latest/download/slippy-find-darwin-arm64 -o slippy-find
chmod +x slippy-find
sudo mv slippy-find /usr/local/bin/
```

### Building from Source

```bash
git clone https://github.com/MyCarrier-DevOps/slippy-find.git
cd slippy-find
go build -o slippy-find .
```

## Usage

```bash
# Basic usage (current directory)
slippy-find

# Specify repository path
slippy-find /path/to/repo

# Increase search depth (default: 25 commits)
slippy-find --depth 50

# Enable verbose logging
slippy-find -v
```

### Output

On success, outputs only the correlation ID to stdout:

```
550e8400-e29b-41d4-a716-446655440000
```

All logs and errors are written to stderr, making the tool suitable for pipeline consumption:

```bash
CORRELATION_ID=$(slippy-find)
```

## Configuration

All configuration is supplied via environment variables. There is no Vault dependency, no ClickHouse connection, and no pipeline-config file — slippy-find talks to slippy-api over HTTP.

### Required

| Variable | Description |
|----------|-------------|
| `SLIPPY_API_URL` | Base URL of the slippy-api service (e.g. `http://slippy-api/v1`). Must include scheme and host. |
| `SLIPPY_API_KEY` | Bearer token sent as `Authorization: Bearer <token>` on every request. |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `SLIPPY_API_IPV4_ONLY` | Force slippy-api dials onto IPv4 (`true`/`false`) | `false` |
| `LOG_LEVEL` | Log level (`debug`, `info`, `error`) | `info` |
| `LOG_APP_NAME` | Application name for log context | `slippy-find` |

### Network Resilience

slippy-find resolves the correlation ID that every downstream step of a routing slip keys on, so a single unreachable-host blip used to fail a whole production release. Calls to slippy-api are therefore retried — but only where a retry can actually help.

- **Retried:** faults where the request never reached a server that then did work on it (connection refused, `network is unreachable`, DNS SERVFAIL), `5xx`, a body truncated mid-read, and a `429` from an intermediary.
- **Not retried:** `200`, `404` (no slip in ancestry), `401`/`403`, any other `4xx`, a body this client cannot decode, an NXDOMAIN host, a certificate that fails verification, and a context the caller cancelled.
- **A deadline is never retried, at any layer.** On a ClickHouse miss, slippy-api's `find-by-commits` falls back to a serial per-commit GitHub GraphQL ancestry walk, and it abandons that walk on client disconnect — restarting from `commits[0]` next time. So a retried timeout redoes the identical work, fails at the identical point, and burns the shared GitHub GraphQL quota once per attempt. One generous attempt beats four truncated ones.
- **`429` depends on who sent it.** slippy-api's own `429` is an authentication-failure lockout whose Fibonacci ladder is *extended* by every request that arrives while locked, so it is terminal; it is identified by the `X-RateLimit-Limit` header it always carries. A `429` without that header came from an intermediary — Cloudflare fronts this API — and is an ordinary throttle, so it is retried honouring `Retry-After`.
- **Backoff:** 4 attempts total (3 retries) with equal jitter — each wait is uniformly random in `[d/2, d)` where `d` doubles from 500ms and is capped at 5s, so the waits run roughly 250-500ms, 500ms-1s, then 1-2s.
- **`Retry-After` is a floor, never a ceiling.** It is honoured in full and jittered *upward*, never trimmed to fit the backoff cap — arriving early is what a rate limiter penalises. The retry budget below is what bounds the wait.
- Each retry is logged at warn level to **stderr**, so a slippy-api that is degrading but still succeeding is visible in workflow logs without contaminating the correlation ID on stdout.

**Latency.** Two bounds, sized independently:

| Bound | Value | Why |
|---|---|---|
| Per attempt | 45s | Sits inside slippy-api's own 60s `WriteTimeout`, which its source documents as leaving "ample room for a capped ancestry walk". A tighter client cap could not serve a legitimately slow walk on *any* attempt, since every attempt gets the same cap. |
| Whole sequence | 50s | Hard ceiling on attempts plus backoff. |
| One `DialContext` | 5s | Name resolution plus every address tried. This is what bounds the retryable failure class. |

`attempts × 45s` is not reachable, because a deadline is terminal — the sequence can only run long by accumulating fast dial failures, which cost 5s each. Worst cases: four dial failures ≈ 22s; a persistent `5xx` ≈ 4s; one slow ancestry walk = 45s in a single attempt.

`SLIPPY_API_IPV4_ONLY=true` narrows `tcp` dials to `tcp4`, skipping a wasted AAAA leg on a host with no IPv6 route.

> **This is an optimisation, not a fix.** It is tempting to read `dial tcp [2606:...]:443: connect: network is unreachable` as "the AAAA leg failed and masked a working A leg", but Go's dialer does not work that way. `net/dial.go` races both families and returns the first success outright; the primary family's error surfaces *only* once the other leg has also finished and failed. So an `ENETUNREACH` reaching the caller means IPv4 was tried and failed too, and forcing `tcp4` would not have turned that failure into a success. The retry is what addresses a transient dial failure.

Nothing in this org sets it, and the canonical GitHub Actions snippet above deliberately does not: the saving is one redundant DNS leg per invocation. It is retained for self-hosted IPv4-only runners. Leave it unset otherwise — on an IPv6-only host it makes every dial fail with `no suitable address found`.

`SLIPPY_API_IPV4_ONLY` is parsed strictly: a value `strconv.ParseBool` rejects (`yes`, `no`, `on`, `off`) is a startup error rather than a silent default, so a typo is reported instead of leaving you believing the flag is on.

### Database Selection (Test vs. Prod)

Database selection is now the responsibility of **slippy-api**, which derives it from its own `K8S_NAMESPACE`. To read slips from the `ci_test` database, point `SLIPPY_API_URL` at the slippy-api deployment in the `slippy-api-test` namespace.

> **Migration note (v1.4.x cutover):** the pre-cutover `WEBHOOK_TARGET=https://test-webhook.mycarrier.tech` selector that routed reads to `ci_test` is gone from this binary. Setting `WEBHOOK_TARGET` on the slippy-find invocation is now a **no-op**. Workflow steps that previously relied on it must instead set `SLIPPY_API_URL` to the test-namespace slippy-api. The same `CLICKHOUSE_*`, `VAULT_*`, and `SLIPPY_PIPELINE_CONFIG` variables are also unused and should be removed from any workflow template that runs slippy-find.

## Example

```bash
export SLIPPY_API_URL="http://slippy-api.svc.cluster.local/v1"
export SLIPPY_API_KEY="$(get-secret slippy-api-token)"

slippy-find
```

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success — correlation ID written to stdout |
| 1 | Error — no slip found, configuration error, auth failure, or transport error |

## Requirements

- Local Git repository with `origin` remote configured
- Network access to a slippy-api instance
- A valid slippy-api bearer token

## Architecture

```
cmd/                    # CLI entry point with Cobra
internal/
  adapters/
    git/                # go-git/v5 adapter for local Git operations
    output/             # stdout writer for correlation ID
    store/              # slippy-api HTTP adapter (SlipAPIAdapter)
  domain/               # Domain interfaces and entities
  infrastructure/
    config/             # Environment-variable configuration loader
  usecases/             # Slip resolution business logic
main.go                 # Production dependency wiring
```

## Development

### Prerequisites

- Go 1.26+
- golangci-lint

### Running Tests

```bash
go test -v -race -coverprofile=coverage.out ./...
```

### Linting

```bash
golangci-lint run -c .github/.golangci.yml
```

### Building

```bash
go build -o slippy-find .
```

## CI/CD

The project includes a GitHub Actions workflow (`.github/workflows/ci.yml`) that runs on every push and pull request to `main`.

### Pipeline Stages

| Stage | Description |
|-------|-------------|
| **Test** | Runs tests with race detection, requires 80% coverage |
| **Lint** | Runs golangci-lint with project configuration |
| **Vuln** | Scans for known vulnerabilities using govulncheck |
| **Release** | Builds binaries and creates GitHub release (main branch only) |

### Release Artifacts

On successful merge to `main`, the pipeline automatically:
- Creates a semantic version tag based on commit messages
- Builds cross-platform binaries (linux/darwin/windows, amd64/arm64)
- Publishes a GitHub Release with all artifacts and checksums
- Updates `proxy.golang.org` for immediate availability via `go install`

## Versioning

This project uses [Semantic Versioning](https://semver.org/) with automatic version bumps based on [Conventional Commits](https://www.conventionalcommits.org/).

### How to Increment the Version

When merging to `main`, the CI pipeline automatically creates a new version tag based on your commit messages:

| Commit Prefix | Version Bump | Example |
|---------------|--------------|---------|
| `fix:` | Patch | v1.0.0 → v1.0.1 |
| `feat:` | Minor | v1.0.0 → v1.1.0 |
| `feat!:` or `BREAKING CHANGE:` | Major | v1.0.0 → v2.0.0 |
| Other | Patch (default) | v1.0.0 → v1.0.1 |

### Commit Message Examples

```bash
# Patch release (bug fix)
git commit -m "fix: handle nil pointer in resolver"

# Minor release (new feature)
git commit -m "feat: add support for custom depth flag"

# Major release (breaking change)
git commit -m "feat!: change output format to JSON"
# or
git commit -m "feat: change output format

BREAKING CHANGE: Output is now JSON instead of plain text"
```

### Release Process

1. Make changes and commit using conventional commit format
2. Push to `main` (directly or via PR merge)
3. CI pipeline automatically:
   - Runs tests, lint, and vulnerability scan
   - Bumps version based on commit messages
   - Builds cross-platform binaries
   - Creates GitHub Release with artifacts
   - Updates `proxy.golang.org` for `go install` users

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
