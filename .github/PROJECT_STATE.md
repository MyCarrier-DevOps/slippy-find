# Project State — slippy-find Application

> **Last Updated:** 2025-02-05
> **Status:** Implementation complete with Vault integration

## Overview

**slippy-find** is a Go CLI application that resolves routing slips using local Git repository commit history. It outputs only the `correlation_id` to stdout for consumption by external systems.

**Key Characteristics:**
- Operates entirely on local Git repositories (no GitHub API calls)
- Uses `go-git/go-git/v5` to walk commit ancestry from HEAD
- Reuses `goLibMyCarrier/slippy` for ClickHouse storage and domain types
- Reuses `goLibMyCarrier/logger` for structured logging to stderr
- All git context (HEAD SHA, branch, repository name) derived from local repository
- Repository name extracted from `origin` remote URL (HTTPS or SSH format)
- **Full dependency injection throughout** - all dependencies are injectable via interfaces

## Implemented Systems

### Completed
- Domain layer with interfaces (`domain/interfaces.go`, `domain/entities.go`)
- Git adapter using go-git/v5 (`adapters/git/gogit.go`)
- Output writer (`adapters/output/writer.go`)
- ClickHouse store adapter (`adapters/store/clickhouse.go`)
- Configuration loading (`infrastructure/config/config.go`)
- Slip resolver use case (`usecases/resolver.go`)
- CLI with proper DI (`cmd/root.go`)
- Production dependency wiring (`main.go`)

### Test Coverage
| Package | Coverage |
|---------|----------|
| cmd | 82.3% |
| adapters/git | 88.7% |
| adapters/output | 100% |
| adapters/store | 100% |
| infrastructure/config | 92% |
| usecases | 100% |

## Recent Changes

### 2025-02-05: Vault Integration for Pipeline Config
- Added HashiCorp Vault integration for loading pipeline configuration
- Uses `goLibMyCarrier/vault` package with AppRole authentication
- Config loading now tries Vault first, falls back to file if Vault not configured
- Added new environment variables: `VAULT_ADDRESS`, `VAULT_ROLE_ID`, `VAULT_SECRET_ID`, `VAULT_PIPELINE_CONFIG_PATH`, `VAULT_PIPELINE_CONFIG_MOUNT`
- Created `VaultClient` interface for dependency injection in tests
- Added `LoadWithVaultClient()` function for testability
- All tests passing with new Vault test coverage (80.3% in config package)

### 2026-02-04: Dependency Injection Refactoring
- Refactored `cmd/root.go` to use proper DI via `Dependencies` struct
- Added factory functions for all dependencies (Logger, Config, GitRepo, SlipFinder, Resolver, OutputWriter)
- Created `NewRootCmdWithDeps()` for testability
- Updated all tests to use mock implementations
- Git adapter now uses local `Logger` interface instead of concrete `logger.Logger`
- Resolver uses `domain.SlipFinder` interface instead of `slippy.SlipStore`
- Created `ClickHouseAdapter` to bridge `slippy.SlipStore` → `domain.SlipFinder`
- All tests passing with race detection
- Lint passing with zero errors

### 2026-02-04: Project Initialization
- Created `.github/PROJECT_STATE.md`
- Created `go.mod` with module path `github.com/MyCarrier-DevOps/slippy-find`
- Scaffolded directory structure per CLEAN architecture

## Current Focus

Dependency injection refactoring complete. All core functionality implemented and tested.

## Architectural Decisions

### AD-001: Local Git Operations Replace GitHub API
- **Decision:** Implement `LocalGitRepository` interface using `go-git/v5` instead of `goLibMyCarrier/slippy.GitHubAPI`
- **Rationale:** Application operates on local repositories; GitHub API calls are unnecessary and would require network access
- **Trade-offs:** Cannot resolve slips for repositories not cloned locally; must have `origin` remote configured

### AD-002: No Repository Override Flag
- **Decision:** Repository name is always derived from local Git `origin` remote; no `--repository` flag
- **Rationale:** Tool is designed exclusively for local repository analysis; overrides could lead to mismatched slip resolution
- **Trade-offs:** Requires valid `origin` remote; fails immediately if not configured

### AD-003: Detached HEAD Handling
- **Decision:** Warn to stderr and continue when HEAD is detached (not on a branch)
- **Rationale:** Slip resolution can still work with commit SHA ancestry; branch name is informational
- **Trade-offs:** Branch-specific slip matching may be degraded

### AD-004: Pipeline Config from Vault (Preferred)
- **Decision:** Pipeline configuration loaded from HashiCorp Vault using AppRole authentication; file-based loading as fallback
- **Rationale:** Centralizes secrets management; follows MyCarrier security practices; eliminates need to distribute config files
- **Implementation:**
  - Uses `goLibMyCarrier/vault` package with AppRole authentication
  - Requires `VAULT_ADDRESS`, `VAULT_ROLE_ID`, `VAULT_SECRET_ID`, `VAULT_PIPELINE_CONFIG_PATH`
  - Falls back to `SLIPPY_PIPELINE_CONFIG` file path if Vault env vars not set
  - Supports pipeline config as JSON string in "config" key or direct field mapping
- **Trade-offs:** Requires Vault infrastructure; additional env vars for Vault auth

### AD-005: Full Dependency Injection
- **Decision:** All external dependencies injected via interfaces; no direct instantiation in business logic
- **Rationale:** Enables comprehensive unit testing via mocks; follows SOLID principles
- **Implementation:** 
  - `cmd/root.go` accepts `Dependencies` struct with factory functions
  - All adapters accept interfaces, not concrete types
  - Domain interfaces defined for: `LocalGitRepository`, `SlipFinder`, `OutputWriter`, `Resolver`, `Logger`
- **Trade-offs:** Additional boilerplate for wiring; `main.go` contains production wiring logic

## Technical Debt / Known Issues

- `main.go` not included in coverage (expected for entry point files)
- `Execute()` function calls `os.Exit()` making it difficult to test

## Next Steps (Not Yet Implemented)

1. ~~Initialize project foundation~~ ✅
2. ~~Define domain interfaces and entities~~ ✅
3. ~~Implement `GoGitRepository` adapter~~ ✅
4. ~~Implement `SlipResolver` use case~~ ✅
5. ~~Implement configuration loading~~ ✅
6. ~~Build CLI and wire dependencies~~ ✅
7. ~~Add comprehensive tests (≥85% coverage)~~ ✅
8. ~~Run validation (lint, test, security checks)~~ ✅
9. Integration testing with real ClickHouse (optional)
10. CI/CD pipeline setup

## Environment Variables Reference

### ClickHouse Configuration
| Variable | Description | Required |
|----------|-------------|----------|
| `CLICKHOUSE_HOSTNAME` | ClickHouse server hostname | Yes |
| `CLICKHOUSE_PORT` | ClickHouse server port | Yes |
| `CLICKHOUSE_USERNAME` | ClickHouse username | Yes |
| `CLICKHOUSE_PASSWORD` | ClickHouse password | Yes |
| `CLICKHOUSE_DATABASE` | ClickHouse database name | No (defaults to "ci") |
| `CLICKHOUSE_SKIP_VERIFY` | Skip TLS verification | No |

### Vault Configuration (Preferred for Pipeline Config)
| Variable | Description | Required |
|----------|-------------|----------|
| `VAULT_ADDRESS` | HashiCorp Vault server address | Yes (if using Vault) |
| `VAULT_ROLE_ID` | AppRole role ID for authentication | Yes (if using Vault) |
| `VAULT_SECRET_ID` | AppRole secret ID for authentication | Yes (if using Vault) |
| `VAULT_PIPELINE_CONFIG_PATH` | Path to pipeline config in Vault KV | Yes (if using Vault) |
| `VAULT_PIPELINE_CONFIG_MOUNT` | Vault KV mount point | No (defaults to "secret") |

### File-based Configuration (Fallback)
| Variable | Description | Required |
|----------|-------------|----------|
| `SLIPPY_PIPELINE_CONFIG` | Path to pipeline config JSON file | Yes (if not using Vault) |

### Logging Configuration
| Variable | Description | Required |
|----------|-------------|----------|
| `LOG_LEVEL` | Logging level (debug, info, error) | No (defaults to "info") |
| `LOG_APP_NAME` | Application name for logs | No (defaults to "slippy-find") |

## CLI Usage

```bash
# Basic usage (current directory)
slippy-find

# Specify repository path
slippy-find /path/to/repo

# Increase search depth
slippy-find --depth 50

# Enable verbose logging
slippy-find -v

```

