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
| `LOG_LEVEL` | Log level (`debug`, `info`, `error`) | `info` |
| `LOG_APP_NAME` | Application name for log context | `slippy-find` |

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
