# Project Instructions for AI Agents

## Build & Test

**Always use Makefile targets, NOT raw `go` / `golangci-lint` commands.** The Makefile encodes the canonical lint config, coverage thresholds, race flags, and tool versions used by CI. Raw `go test ./...` may pass while `make test` (and CI) fail because flags differ.

```bash
make lint            # golangci-lint w/ repo config
make test            # full test suite w/ race + coverage
make fmt             # formatters
make tidy            # go mod tidy
make check-sec       # gosec scan
make bump            # version bump helper
make build           # build binary
make clean           # remove build artifacts
```

Discover targets: `grep -E "^[a-z_-]+:" Makefile`.

**Quick iteration:** `go build ./...` + `go vet ./...` acceptable. **Final gate before commit MUST be `make lint && make test`** — CI compares against Makefile output.

**For subagents:** brief them w/ Makefile targets explicitly. Don't let them substitute raw commands.

## Conventions

- Conventional Commits — see `.claude/rules/smart-commits.md`
- No `--no-verify` — fix root cause if hook fails
