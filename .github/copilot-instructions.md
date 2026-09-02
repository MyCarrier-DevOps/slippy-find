# slippy-find AI Development Instructions

## CRITICAL: Read Project History First

### At Session Start Before making ANY changes, read [CHANGELOG.md](../CHANGELOG.md), "Project history" section.
- Understand current state, recent changes, and pending work
- Review any in-progress items or architectural decisions
- Current implementation status and architecture
- Domain-specific patterns (shadow mode, correlation ID flow, slip resolution)
- Code examples and anti-patterns
- Environment variable reference

---

## Required Reading

1. **[CHANGELOG.md](../CHANGELOG.md)**, "Project history" section - Project context, architecture, domain patterns
2. Go conventions are owned by the **go-devkit** plugin (see the go-devkit
   block in [CLAUDE.md](../CLAUDE.md)) - naming, error handling, package
   layout, concurrency, HTTP clients, testing, security

---

## Core Rules

### Research First, Never Assume
- Check existing code and tests before implementing
- Use Context7 MCP for third-party library documentation
- Never guess at function signatures or import paths

### No Incomplete Code
- **NEVER** leave TODO stubs, placeholder implementations, or "not yet implemented" code
- Every function must be fully implemented before moving on

### Dependency Injection
- Accept interfaces as parameters, return concrete types
- All external dependencies must be injectable for testing

### Test-Driven Development
Follow TDD strictly: **Red → Green → Refactor**

### Code Quality
- All code must pass `golangci-lint` with zero errors
- No `//nolint` directives without explicit user approval
- Minimum 80% test coverage required

---

## Validation (Required Before Completion)

```bash
go fmt ./...
go mod tidy
golangci-lint run -c .github/.golangci.yml          # Zero errors required
go test -v -race -coverprofile=coverage.out ./...   # 85%+ coverage required
go build -o slippy ./cmd/slippy
```

---

## Project State Documentation

**You MUST maintain the "Project history" section of [CHANGELOG.md](../CHANGELOG.md) as working memory for this project.**

### At Session Start
- **Read CHANGELOG.md's "Project history" section** to understand current state, recent changes, and pending work
- Review any in-progress items or architectural decisions

### During Work
Update CHANGELOG.md's "Project history" section whenever:
- Implementing new features or systems
- Making architectural decisions or changes
- Discovering technical debt or issues
- Completing significant milestones
- User provides direction that affects project structure

### Required Sections
```markdown
# Project State — Slippy Application

> **Last Updated:** [Date]
> **Status:** [Brief current state summary]

## Overview
[What this application does, key characteristics]

## Implemented Systems
[Breakdown of completed functionality by component]

## Recent Changes
[What changed in recent sessions, new components/systems/patterns]

## Current Focus
[What's being worked on now, immediate next steps]

## Architectural Decisions
[Key design choices made, rationale, tradeoffs]

## Technical Debt / Known Issues
[Outstanding problems, things to fix later]

## Next Steps (Not Yet Implemented)
[Planned features, improvements, user requests pending]
```

### Update Discipline
- Keep entries **concise but specific** (reference file paths, function names)
- **Date entries** in Recent Changes section
- Remove outdated items from "Next Steps" as they're completed
