# Go Development Instructions

Follow idiomatic Go practices and community standards when writing Go code. These instructions are based on [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), and [Google's Go Style Guide](https://google.github.io/styleguide/go/).

## General Instructions

- Write simple, clear, and idiomatic Go code
- Favor clarity and simplicity over cleverness
- Follow the principle of least surprise
- Keep the happy path left-aligned (minimize indentation)
- Return early to reduce nesting
- Prefer early return over if-else chains; use `if condition { return }` pattern to avoid else blocks
- Make the zero value useful
- Write self-documenting code with clear, descriptive names
- Document exported types, functions, methods, and packages
- Use Go modules for dependency management
- Leverage the Go standard library instead of reinventing the wheel (e.g., use `strings.Builder` for string concatenation, `filepath.Join` for path construction)
- Prefer standard library solutions over custom implementations when functionality exists
- Write comments in English by default; translate only upon user request
- Avoid using emoji in code and comments

## Naming Conventions

### Packages

- Use lowercase, single-word package names
- Avoid underscores, hyphens, or mixedCaps
- Choose names that describe what the package provides, not what it contains
- Avoid generic names like `util`, `common`, or `base`
- Package names should be singular, not plural

#### Package Declaration Rules (CRITICAL):
- **NEVER duplicate `package` declarations** - each Go file must have exactly ONE `package` line
- When editing an existing `.go` file:
  - **PRESERVE** the existing `package` declaration - do not add another one
  - If you need to replace the entire file content, start with the existing package name
- When creating a new `.go` file:
  - **BEFORE writing any code**, check what package name other `.go` files in the same directory use
  - Use the SAME package name as existing files in that directory
  - If it's a new directory, use the directory name as the package name
  - Write **exactly one** `package <name>` line at the very top of the file

### Variables and Functions

- Use mixedCaps or MixedCaps (camelCase) rather than underscores
- Keep names short but descriptive
- Use single-letter variables only for very short scopes (like loop indices)
- Exported names start with a capital letter
- Unexported names start with a lowercase letter
- Avoid stuttering (e.g., avoid `http.HTTPServer`, prefer `http.Server`)

### Interfaces

- Name interfaces with -er suffix when possible (e.g., `Reader`, `Writer`, `Formatter`)
- Single-method interfaces should be named after the method (e.g., `Read` → `Reader`)
- Keep interfaces small and focused

### Constants

- Use MixedCaps for exported constants
- Use mixedCaps for unexported constants
- Group related constants using `const` blocks
- Consider using typed constants for better type safety

## Code Style and Formatting

### Formatting

- Always use `gofmt` to format code
- Use `goimports` to manage imports automatically
- Keep line length reasonable (golangci `golines` max: 120)
- Add blank lines to separate logical groups of code

### Comments

- Strive for self-documenting code; prefer clear variable names, function names, and code structure over comments
- Write comments only when necessary to explain complex logic, business rules, or non-obvious behavior
- Write comments in complete sentences in English by default
- Start sentences with the name of the thing being described
- Package comments should start with "Package [name]"
- Use line comments (`//`) for most comments
- Use block comments (`/* */`) sparingly, mainly for package documentation
- Document why, not what, unless the what is complex

### Error Handling

- Check errors immediately after the function call
- Don't ignore errors using `_` unless you have a good reason (document why)
- Wrap errors with context using `fmt.Errorf` with `%w` verb
- Create custom error types when you need to check for specific errors
- Place error returns as the last return value
- Name error variables `err`
- Keep error messages lowercase and don't end with punctuation

## Architecture and Project Structure

### Package Organization

- Follow standard Go project layout conventions
- Keep `main` packages in `cmd/` directory
- Put reusable packages in `pkg/` or `internal/`
- Use `internal/` for packages that shouldn't be imported by external projects
- Group related functionality into packages
- Avoid circular dependencies

### Dependency Management

- Use Go modules (`go.mod` and `go.sum`)
- Keep dependencies minimal
- Regularly update dependencies for security patches
- Use `go mod tidy` to clean up unused dependencies

## Type Safety and Language Features

### Type Definitions

- Define types to add meaning and type safety
- Use struct tags for JSON, XML, database mappings
- Prefer explicit type conversions
- Use type assertions carefully and check the second return value
- Prefer generics over unconstrained types; when an unconstrained type is truly needed, use the predeclared alias `any` instead of `interface{}` (Go 1.18+)

### Pointers vs Values

- Use pointer receivers for large structs or when you need to modify the receiver
- Use value receivers for small structs and when immutability is desired
- Be consistent within a type's method set
- Consider the zero value when choosing pointer vs value receivers

### Interfaces and Composition

- Accept interfaces, return concrete types
- Keep interfaces small (1-3 methods is ideal)
- Use embedding for composition
- Define interfaces close to where they're used, not where they're implemented
- Don't export interfaces unless necessary

## Concurrency

### Goroutines

- Be cautious about creating goroutines in libraries; prefer letting the caller control concurrency
- Always know how a goroutine will exit
- Use `sync.WaitGroup` or channels to wait for goroutines
- Avoid goroutine leaks by ensuring cleanup

### Channels

- Use channels to communicate between goroutines
- Don't communicate by sharing memory; share memory by communicating
- Close channels from the sender side, not the receiver
- Use buffered channels when you know the capacity
- Use `select` for non-blocking operations

### Synchronization

- Use `sync.Mutex` for protecting shared state
- Keep critical sections small
- Use `sync.RWMutex` when you have many readers
- Use `sync.Once` for one-time initialization

## Error Handling Patterns

### Creating Errors

- Use `errors.New` for simple static errors
- Use `fmt.Errorf` for dynamic errors
- Create custom error types for domain-specific errors
- Export error variables for sentinel errors
- Use `errors.Is` and `errors.As` for error checking

### Error Propagation

- Add context when propagating errors up the stack
- Don't log and return errors (choose one)
- Handle errors at the appropriate level

## API Design

### HTTP Handlers

- Use `http.HandlerFunc` for simple handlers
- Implement `http.Handler` for handlers that need state
- Use middleware for cross-cutting concerns
- Set appropriate status codes and headers
- Prefer the enhanced `net/http` `ServeMux` with pattern-based routing (Go 1.22+)

### JSON APIs

- Use struct tags to control JSON marshaling
- Validate input data
- Use pointers for optional fields
- Handle JSON errors appropriately

### HTTP Clients

- Keep the client struct focused on configuration only; do not store per-request state
- Construct a fresh `*http.Request` per method invocation
- Ensure the underlying `*http.Client` is configured with sensible timeouts
- Always `defer resp.Body.Close()`

## Testing

### Test Organization

- Keep tests in the same package (white-box testing) or use `_test` package suffix for black-box testing
- Name test files with `_test.go` suffix
- Place test files next to the code they test

### Writing Tests

- Use table-driven tests for multiple test cases
- Name tests descriptively
- Use subtests with `t.Run` for better organization
- Test both success and error cases

### Test Helpers

- Mark helper functions with `t.Helper()`
- Create test fixtures for complex setup
- Clean up resources using `t.Cleanup()`

## Security Best Practices

### Input Validation

- Validate all external input
- Use strong typing to prevent invalid states
- Sanitize data before using in SQL queries
- Be careful with file paths from user input

### Cryptography

- Use standard library crypto packages
- Don't implement your own cryptography
- Use `crypto/rand` for random number generation
- Store passwords using bcrypt, scrypt, or argon2
- Use TLS for network communication

## Documentation

- Document all exported symbols with clear, concise explanations
- Start documentation with the symbol name
- Use examples in documentation when helpful

## Tools and Development Workflow

### Essential Tools

- `go fmt`, `go vet`, `golangci-lint`, `go test`, `go mod`, `govulncheck`

### Development Practices

- Run `make lint test check-sec` before committing
- Use the `pre-commit` hooks shipped with this template
- Keep commits focused and atomic
- Write meaningful commit messages

## Common Pitfalls to Avoid

- Not checking errors
- Ignoring race conditions
- Creating goroutine leaks
- Not using defer for cleanup
- Modifying maps concurrently
- Not understanding nil interfaces vs nil pointers
- Forgetting to close resources (files, connections)
- Using global variables unnecessarily
- Not considering the zero value of types
- **Creating duplicate `package` declarations** - this is a compile error
