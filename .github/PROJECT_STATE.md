# Project State — slippy-find Application

> **Last Updated:** 2026-05-13
> **Status:** Production ready; cut over to slippy-api HTTP client (sole-authority Phase 1c)

## Overview

**slippy-find** is a Go CLI application that resolves routing slips using local Git repository commit history. It outputs only the `correlation_id` to stdout for consumption by external systems.

**Key Characteristics:**
- Operates entirely on local Git repositories (no GitHub API calls)
- Uses `go-git/go-git/v5` to walk commit ancestry from HEAD
- Resolves slips by calling `POST /slips/find-by-commits` on **slippy-api** via the generated `slippy-api/slippy-client` (bearer-token auth, 30s timeout)
- Uses `goLibMyCarrier/logger` for structured logging to stderr
- All git context (HEAD SHA, branch, repository name) derived from local repository
- Repository name extracted from `origin` remote URL (HTTPS or SSH format)
- **Full dependency injection throughout** — all dependencies are injectable via interfaces

## Implemented Systems

### Completed
- Domain layer with interfaces (`internal/domain/interfaces.go`, `internal/domain/entities.go`)
- Git adapter using go-git/v5 (`internal/adapters/git/gogit.go`)
- Output writer (`internal/adapters/output/writer.go`)
- slippy-api HTTP store adapter (`internal/adapters/store/slipapi.go`)
- Configuration loading (`internal/infrastructure/config/config.go`)
- Slip resolver use case (`internal/usecases/resolver.go`)
- CLI with proper DI (`cmd/root.go`)
- Production dependency wiring (`main.go`)

### Test Coverage
| Package | Coverage |
|---------|----------|
| cmd | ~82% |
| adapters/git | ~89% |
| adapters/output | 100% |
| adapters/store | 100% |
| infrastructure/config | ~92% |
| usecases | 100% |

## Recent Changes

### 2026-05-13: Cut over to slippy-api HTTP client (sole-authority Phase 1c)
- Replaced `ClickHouseAdapter` (direct `goLibMyCarrier/slippy` + ClickHouse connection) with `SlipAPIAdapter` that calls `POST /slips/find-by-commits` via `slippy-api/slippy-client v1.4.3`.
- Dropped `goLibMyCarrier/{clickhouse,slippy,vault}` from `go.mod`.
- Simplified configuration: required `SLIPPY_API_URL` + `SLIPPY_API_KEY`. Dropped `CLICKHOUSE_*`, `VAULT_*`, `SLIPPY_PIPELINE_CONFIG`, `WEBHOOK_TARGET`, `SLIPPY_DATABASE`.
- Database selection (`ci` vs `ci_test`) now lives in slippy-api: it derives the database from `K8S_NAMESPACE`. Workflow steps that previously routed reads to `ci_test` via `WEBHOOK_TARGET` must now set `SLIPPY_API_URL` to the slippy-api deployment in the `slippy-api-test` namespace.
- Bounded HTTP client with `Timeout: 30s` so a misconfigured `SLIPPY_API_URL` fails fast.
- Silenced the wrapper client's default `slog` logger so stderr stays "warnings/errors only" per the README contract.
- `SLIPPY_API_URL` is validated with `url.Parse` at the config boundary (scheme + host required) so typos surface immediately rather than as opaque relative-URL errors inside the generated client.
- Non-200/404 responses now surface the RFC 7807 `detail`/`title` from the API; 401/403 produce an explicit "authentication failed — check `SLIPPY_API_KEY`" hint.
- Added tests: bearer header assertion, request body shape decode, 401 error-detail surfacing, invalid-URL rejection, and wiring of `AppConfig.SlippyAPIURL`/`SlippyAPIKey` through to `SlipFinderFactory`.

### 2026-03-13: Fix Detached Merge Commit Ancestry Walk (CI PR Checkout)
- **Root cause:** GitHub Actions `actions/checkout` creates a merge commit in detached HEAD state for PRs. Parent 0 is the base branch, parent 1 is the feature branch. The first-parent-only walk was following only the base branch, never reaching feature branch commits where the routing slip was created.
- Modified `GetCommitAncestry` in `internal/adapters/git/gogit.go` to detect detached merge commits and walk **all** parent chains independently (each following first-parent).
- Extracted `walkFirstParent` helper method for reusable first-parent chain walking.
- Added integration test `TestGoGitRepository_GetCommitAncestry_DetachedMergeCommit` verifying both base and feature branch commits are found.

### 2026-02-25: CI Tooling Version Alignment
- Updated GitHub Actions workflow to use `GO_VERSION: '1.26'` and `golangci-lint v2.10.1`.

## Architectural Decisions

### AD-001: Local Git Operations Replace GitHub API
- **Decision:** Implement `LocalGitRepository` interface using `go-git/v5` instead of any GitHub API.
- **Rationale:** Application operates on local repositories; GitHub API calls are unnecessary and would require network access.
- **Trade-offs:** Cannot resolve slips for repositories not cloned locally; must have `origin` remote configured.

### AD-002: No Repository Override Flag
- **Decision:** Repository name is always derived from local Git `origin` remote; no `--repository` flag.
- **Rationale:** Tool is designed exclusively for local repository analysis; overrides could lead to mismatched slip resolution.
- **Trade-offs:** Requires valid `origin` remote; fails immediately if not configured.

### AD-003: Detached HEAD Handling
- **Decision:** Warn to stderr and continue when HEAD is detached (not on a branch).
- **Rationale:** Slip resolution can still work with commit SHA ancestry; branch name is informational.
- **Enhancement (2026-03-13):** When HEAD is a detached **merge commit** (typical of CI PR checkout), walk all parent chains so both base and feature branch commits are searched.
- **Trade-offs:** Branch-specific slip matching may be degraded; merge commit walks return more commits than depth (up to `1 + depth * num_parents`).

### AD-004: slippy-api HTTP Client (replaces direct ClickHouse + Vault)
- **Decision:** Resolve slips by calling slippy-api over HTTP using the generated `slippy-api/slippy-client`. Authenticate with a bearer token; no Vault, no ClickHouse, no pipeline-config file.
- **Rationale:** Phase 1c of the sole-authority migration. slippy-find should never own a direct database connection; all reads go through slippy-api so a single service owns slip storage semantics (including the `ci` vs `ci_test` database split, derived from slippy-api's own `K8S_NAMESPACE`).
- **Implementation:**
  - `internal/adapters/store/slipapi.go` wraps `slippyclient.WrappedClient`.
  - 30s HTTP timeout, bearer auth, RFC 7807 problem-detail extraction on errors.
  - URL validated with `url.Parse` at the config boundary.
- **Trade-offs:** Requires network reachability to slippy-api; adds a hop versus direct ClickHouse access.

### AD-005: Full Dependency Injection
- **Decision:** All external dependencies injected via interfaces; no direct instantiation in business logic.
- **Rationale:** Enables comprehensive unit testing via mocks; follows SOLID principles.
- **Implementation:**
  - `cmd/root.go` accepts `Dependencies` struct with factory functions.
  - All adapters accept interfaces, not concrete types.
  - Domain interfaces defined for: `LocalGitRepository`, `SlipFinder`, `OutputWriter`, `Resolver`, `Logger`.
- **Trade-offs:** Additional boilerplate for wiring; `main.go` contains production wiring logic.

## Technical Debt / Known Issues

- `main.go` not included in coverage (expected for entry point files).
- `Execute()` function calls `os.Exit()` making it difficult to test.

## Environment Variables Reference

### Required
| Variable | Description |
|----------|-------------|
| `SLIPPY_API_URL` | Base URL of the slippy-api service (e.g. `http://slippy-api/v1`). Must include scheme and host. |
| `SLIPPY_API_KEY` | Bearer token for authenticating slip read requests. |

### Optional
| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level (`debug`, `info`, `error`) | `info` |
| `LOG_APP_NAME` | Application name for logs | `slippy-find` |

### Retired (no longer read — remove from workflow templates)
- `CLICKHOUSE_HOSTNAME`, `CLICKHOUSE_PORT`, `CLICKHOUSE_USERNAME`, `CLICKHOUSE_PASSWORD`, `CLICKHOUSE_SKIP_VERIFY`
- `VAULT_ADDRESS`, `VAULT_ROLE_ID`, `VAULT_SECRET_ID`, `VAULT_PIPELINE_CONFIG_PATH`, `VAULT_PIPELINE_CONFIG_MOUNT`
- `SLIPPY_PIPELINE_CONFIG`
- `SLIPPY_DATABASE`
- `WEBHOOK_TARGET` (database selection moved to slippy-api; route reads to the test namespace's slippy-api instead)

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
