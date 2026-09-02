# Project Instructions for AI Agents

This repo's Go workflow — the idiomatic-Go conventions, the RED-test-first delivery loop, the security preflight, and the coverage gate — is owned by the **go-devkit** plugin (an apm dependency declared in [apm.yml](apm.yml); its machinery is installed outside the repo tree under `apm_modules/` via `apm install`). The go-devkit block below is the authoritative description of that workflow and is kept in sync by `/go-repo-init` — do not edit it by hand. The sections after it record the project-specific facts the plugin cannot know: this repo's module layout, coverage policy, and house rules.

<!-- BEGIN go-devkit -->
## Development workflow (go-devkit)

This repository uses the **go-devkit** Claude Code plugin. The idiomatic Go
conventions this project enforces (naming, error handling, package layout,
concurrency, HTTP clients, testing, security) live in the plugin and are loaded
by the `go-tdd` skill — read them before writing Go code.

For features and bugfixes, use the **`go-tdd`** skill — it drives the full loop:

1. **RED test first (code changes only).** Write a failing table-driven test
   before the implementation, then make it pass, then refactor — confirm it
   fails for the intended reason first. This does **not** apply to meta changes
   (renaming the app / `APPLICATION`, config, docs, dependency bumps); make those
   directly and verify with `/go-verify`.
2. **Preflight before coding.** Run `/go-preflight` (`make check-sec`) after
   planning and before implementing. If `govulncheck` flags a Go standard-library
   CVE, upgrade the toolchain (`brew upgrade go`, or `mise use -g go@latest`)
   before continuing; if it flags a dependency, `make bump`.
3. **Verify after.** Run `/go-verify` when the task is done — it runs `make fmt`,
   `make lint`, `make test`, and the plugin's coverage gate (which reads the CI
   `threshold-total` live so local and CI never drift). On non-main branches it
   finishes with mutation testing (`make mutation`, `mutest -diff origin/main`) —
   surviving mutants mean missing assertions; add tests rather than skip.
4. **The commit is gated.** In checkouts armed by `/go-repo-init`, a pre-commit
   hook re-runs fmt, lint, test, and (on non-main branches, when the run would
   judge exactly what the commit stages) mutation before any `git commit`, and
   blocks the commit until they pass. If the hook reports it skipped mutation,
   that is not a pass — deal with the reason it names. Do not try to bypass
   the gate — fix the failure it reports.

**Pin the Go version to a full patch release, and keep it in sync.** The `go`
directive in the module's `go.mod` (e.g. `go 1.26.5`, not `go 1.26`) and the
builder image in the Dockerfile (`golang:1.26.5`) must name the same patch. CI
intentionally floats on the patch level (`go-version: "1.26"`); bump it by hand
for a new minor or major.
<!-- END go-devkit -->

*Project note: this repo pins `go 1.26` (minor level) in go.mod and CI — the
full-patch prescription above assumes a Dockerfile pairing this repo doesn't
have; upstream doc fix tracked.*

## Module layout

Single Go module at the **repository root** (`./go.mod`, module
`github.com/MyCarrier-DevOps/slippy-find`). There is no `app/` directory — the
Makefile's `APPLICATION` is therefore `.`.

| Path | Responsibility |
| --- | --- |
| `main.go` | Production dependency wiring and entrypoint. |
| `cmd/root.go` | Cobra CLI with dependency injection (`Dependencies` struct with factory functions). |
| `internal/domain/` | Interfaces (`LocalGitRepository`, `SlipFinder`, `OutputWriter`, `Resolver`, `Logger`) and entities. Zero external deps. |
| `internal/adapters/git/` | `go-git/v5`-backed implementation of `LocalGitRepository`; walks commit ancestry, handles detached/merge-commit HEAD. |
| `internal/adapters/output/` | Writes the resolved `correlation_id` to stdout. |
| `internal/adapters/store/` | `SlipAPIAdapter` — wraps `slippy-api/slippy-client` to call `POST /slips/find-by-commits` over HTTP (bearer auth, bounded retry). |
| `internal/adapters/logger/` | `goLibMyCarrier/logger`-backed structured logging to stderr. |
| `internal/infrastructure/config/` | Environment-variable configuration loading (`SLIPPY_API_URL`, `SLIPPY_API_KEY`, `LOG_LEVEL`, `LOG_APP_NAME`). |
| `internal/usecases/resolver.go` | Slip resolver use case tying the interfaces together. |

## Commands

**Always use Makefile targets, NOT raw `go` / `golangci-lint` commands.** The
Makefile encodes the canonical lint config, tool versions, and the coverage
threshold (`coverage-check`, 80%) used by CI, and now also gates the vuln
scan (`check-sec`). Raw `go test ./...` may pass while `make test` (and CI)
fail because flags differ. CI's authoritative gates live in `ci.yml`: the
`test` job's 80% coverage-action gate and the `vuln` job, which now invoke
`make install-tools`/`make check-sec` so the tool-version source of truth is
the Makefile, not a duplicated inline install. `make help` lists everything;
the ones that matter day to day:

```bash
make test            # unit tests with race + coverage
make coverage-check  # test, then assert total coverage >= 80%
make lint            # golangci-lint, config at .github/.golangci.yml
make fmt             # gofmt-equivalent formatters via golangci-lint
make tidy            # go mod tidy
make check-sec       # govulncheck
make mutation        # mutest against origin/main
make mutation-all    # mutest across the whole module (weekly CI audit)
make ci              # lint + coverage-check + build + check-sec
make clean           # remove build artifacts
make help            # list all targets
make run ARGS="--depth 50"
make build           # build binary
```

Discover targets: `grep -E "^[a-z_-]+:" Makefile`.

**Quick iteration:** `go build ./...` + `go vet ./...` acceptable. **Final
gate before commit MUST be `make ci`** — CI compares against Makefile output.

## Coverage policy

**80% overall**, enforced by `threshold-total: 80` in the `test` job of
`.github/workflows/ci.yml`. The go-devkit coverage gate (run by `/go-verify`
via the plugin's `coverage-gate.sh`) reads that CI value live, so `make test`
(which writes `coverage.out`) and CI never drift as long as the workflow
threshold stays in sync with reality. This threshold is this repo's existing
gate — do **not** raise it to the 85% used by other go-devkit repos without a
separate, deliberate decision.

## Project house rules

- **Git access goes through the domain interfaces**
  (`internal/domain/interfaces.go`), never directly against `go-git`. That
  seam is what makes `cmd/root.go`'s dependency injection testable —
  bypassing it breaks mocking.
- **HTTP clients hold only configuration, never per-request state.** The
  `SlipAPIAdapter` (`internal/adapters/store/slipapi.go`) must not store or
  cache `*http.Request` across calls; build the request fresh per method
  invocation and always close response bodies.
- **`os.Exit()` lives only in `main.go`/`Execute()`.** Keep it out of
  testable business logic; `main.go` is intentionally excluded from the
  coverage denominator as an entry point.
- **Table-driven tests** for anything with a clear input/output relation. The
  RED-first rule from the go-devkit block above still applies to each case.
- **`WaitGroup.Go`** (Go 1.25+) is preferred over the classic `Add`/`Done`
  pattern for new concurrent code, matching the module's `go` directive.
- **Do not hand-edit the go-devkit block above.** Change it in the plugin and
  re-run `/go-repo-init`.

## Conventions

- Conventional Commits — see `.claude/rules/smart-commits.md`
- No `--no-verify` — fix root cause if hook fails

## History

Project history prior to the go-devkit onboarding (implemented systems,
architectural decisions, environment variables) lives in
[CHANGELOG.md](CHANGELOG.md), imported from the retired
`.github/PROJECT_STATE.md`.
