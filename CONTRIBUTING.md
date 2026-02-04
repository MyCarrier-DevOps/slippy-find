# Contributing to slippy-find

Thank you for your interest in contributing to slippy-find! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions. We welcome contributors of all experience levels.

## Getting Started

### Prerequisites

- Go 1.25.5 or later
- golangci-lint
- Access to test infrastructure (ClickHouse, Vault) for integration testing

### Setting Up Your Development Environment

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/slippy-find.git
   cd slippy-find
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/MyCarrier-DevOps/slippy-find.git
   ```
4. Install dependencies:
   ```bash
   go mod download
   ```

## Development Workflow

### Branching Strategy

- Create feature branches from `main`
- Use descriptive branch names: `feature/add-xyz`, `fix/issue-123`, `docs/update-readme`

### Making Changes

1. Create a new branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes following the coding standards below

3. Run tests:
   ```bash
   go test -v -race ./...
   ```

4. Run linting:
   ```bash
   golangci-lint run
   ```

5. Commit your changes with clear, descriptive messages:
   ```bash
   git commit -m "feat: add support for XYZ"
   ```

### Commit Message Format

We follow conventional commits:

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `test:` - Test additions or modifications
- `refactor:` - Code refactoring without feature changes
- `chore:` - Maintenance tasks

Examples:
```
feat: add support for custom Vault mount paths
fix: handle detached HEAD state correctly
docs: update configuration examples in README
test: add integration tests for ClickHouse adapter
```

## Coding Standards

### Go Guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go) guidelines
- Use `gofmt` for formatting (enforced by CI)
- All exported functions must have documentation comments
- Maintain minimum 85% test coverage for new code

### Architecture

This project follows Clean Architecture principles:

```
cmd/           # CLI entry point
internal/
  adapters/    # External system adapters (git, database, output)
  domain/      # Domain interfaces and entities
  infrastructure/  # Configuration and cross-cutting concerns
  usecases/    # Business logic
```

### Dependency Injection

- Accept interfaces as parameters, return concrete types
- All external dependencies must be injectable for testing
- Define interfaces in the package that uses them (not the implementer)

### Testing

- Write tests for all new functionality
- Use table-driven tests where appropriate
- Mock external dependencies using interfaces
- Run tests with race detection: `go test -race ./...`

## Pull Request Process

1. Ensure all tests pass and linting is clean
2. Update documentation if needed
3. Create a pull request with:
   - Clear title describing the change
   - Description of what and why (not how)
   - Reference to any related issues
4. Request review from maintainers
5. Address review feedback
6. Once approved, a maintainer will merge your PR

### PR Checklist

- [ ] Tests pass (`go test -v -race ./...`)
- [ ] Linting passes (`golangci-lint run`)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventional commits
- [ ] No breaking changes (or clearly documented if unavoidable)

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

- Go version (`go version`)
- Operating system and version
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs or error messages

### Feature Requests

For feature requests, please describe:

- The problem you're trying to solve
- Your proposed solution (if any)
- Any alternatives you've considered

## Questions?

If you have questions about contributing, feel free to:

- Open a GitHub issue with the `question` label
- Review existing issues and pull requests for context

## License

By contributing to slippy-find, you agree that your contributions will be licensed under the MIT License.
