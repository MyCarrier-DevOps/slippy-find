# slippy-find

A Go CLI application that resolves routing slips from local Git repository commit history, outputting only the `correlation_id` for consumption by external systems.

## Overview

`slippy-find` walks the commit ancestry of a local Git repository to find the most recent routing slip associated with any commit in the history. It queries a ClickHouse database (using `goLibMyCarrier/slippy`) to match commits and returns the correlation ID for pipeline orchestration.

### Key Features

- **Local Git operations only** — No GitHub API calls; works entirely with local repositories
- **Commit ancestry walking** — Uses `go-git/v5` to traverse commit history from HEAD
- **ClickHouse integration** — Queries slip store via `goLibMyCarrier/slippy`
- **Vault integration** — Loads pipeline configuration from HashiCorp Vault using AppRole authentication
- **Clean architecture** — Full dependency injection for testability

## Installation

### Using `go install`

```bash
go install github.com/MyCarrier-DevOps/slippy-find@latest
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

### Pipeline Configuration (Required)

Pipeline configuration can be loaded from **HashiCorp Vault** (preferred) or a **local file** (fallback).

#### Option 1: Vault (Preferred)

Set the following environment variables:

| Variable | Description | Required |
|----------|-------------|----------|
| `VAULT_ADDRESS` | HashiCorp Vault server address | Yes |
| `VAULT_ROLE_ID` | AppRole role ID for authentication | Yes |
| `VAULT_SECRET_ID` | AppRole secret ID for authentication | Yes |
| `VAULT_PIPELINE_CONFIG_PATH` | Path to pipeline config in Vault KV (supports `path#key` syntax) | Yes |
| `VAULT_PIPELINE_CONFIG_MOUNT` | KV mount point | No (default: `secret`) |

**Path Syntax:**

The `VAULT_PIPELINE_CONFIG_PATH` supports an optional key suffix using `#` to specify which key in the secret contains the pipeline config:

- `ci/slippy/pipeline` — Uses the default `config` key
- `ci/slippy/pipeline#config` — Explicitly uses the `config` key
- `DevOps/slippy/config#mykey` — Uses the `mykey` key

The pipeline config in Vault can be stored as:
- A JSON string in the specified key (or `config` by default)
- Direct field mapping in the secret (fallback)

#### Option 2: Local File (Fallback)

If Vault environment variables are not set, falls back to file-based configuration:

| Variable | Description | Required |
|----------|-------------|----------|
| `SLIPPY_PIPELINE_CONFIG` | Path to pipeline config JSON file | Yes |

### ClickHouse Configuration (Required)

| Variable | Description | Required |
|----------|-------------|----------|
| `CLICKHOUSE_HOSTNAME` | ClickHouse server hostname | Yes |
| `CLICKHOUSE_PORT` | ClickHouse server port | Yes |
| `CLICKHOUSE_USERNAME` | ClickHouse username | Yes |
| `CLICKHOUSE_PASSWORD` | ClickHouse password | Yes |
| `CLICKHOUSE_SKIP_VERIFY` | Skip TLS verification | No |

### Slip Storage Configuration (Optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `SLIPPY_DATABASE` | ClickHouse database name for slip storage | `ci` |

### Logging Configuration (Optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Log level (`debug`, `info`, `error`) | `info` |
| `LOG_APP_NAME` | Application name for logs | `slippy-find` |

## Example Configuration

### Using Vault

```bash
export VAULT_ADDRESS="https://vault.example.com"
export VAULT_ROLE_ID="your-role-id"
export VAULT_SECRET_ID="your-secret-id"
export VAULT_PIPELINE_CONFIG_PATH="ci/slippy/pipeline-config#config"

export CLICKHOUSE_HOSTNAME="clickhouse.example.com"
export CLICKHOUSE_PORT="9440"
export CLICKHOUSE_USERNAME="default"
export CLICKHOUSE_PASSWORD="your-password"

slippy-find
```

### Using Local File

```bash
export SLIPPY_PIPELINE_CONFIG="/path/to/pipeline.json"

export CLICKHOUSE_HOSTNAME="clickhouse.example.com"
export CLICKHOUSE_PORT="9440"
export CLICKHOUSE_USERNAME="default"
export CLICKHOUSE_PASSWORD="your-password"

slippy-find
```

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success — correlation ID written to stdout |
| 1 | Error — no slip found or configuration/connection error |

## Requirements

- Local Git repository with `origin` remote configured
- ClickHouse database with slip store schema
- Pipeline configuration (via Vault or local file)

## Architecture

```
cmd/                    # CLI entry point with Cobra
internal/
  adapters/
    git/                # go-git/v5 adapter for local Git operations
    output/             # stdout writer for correlation ID
    store/              # ClickHouse adapter bridging slippy.SlipStore
  domain/               # Domain interfaces and entities
  infrastructure/
    config/             # Configuration loading (Vault + file)
  usecases/             # Slip resolution business logic
main.go                 # Production dependency wiring
```

## Development

### Prerequisites

- Go 1.21+
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