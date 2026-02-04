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

Build from source:

```bash
cd ci/slippy-find
go build -o slippy .
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
| `VAULT_PIPELINE_CONFIG_PATH` | Path to pipeline config in Vault KV | Yes |
| `VAULT_PIPELINE_CONFIG_MOUNT` | KV mount point | No (default: `secret`) |

The pipeline config in Vault can be stored as:
- A JSON string in a `config` key
- Direct field mapping in the secret

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
| `CLICKHOUSE_DATABASE` | ClickHouse database name | No (default: `ci`) |
| `CLICKHOUSE_SKIP_VERIFY` | Skip TLS verification | No |

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
export VAULT_PIPELINE_CONFIG_PATH="ci/slippy/pipeline-config"

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
go build -o slippy .
```

## License

Internal MyCarrier tool.