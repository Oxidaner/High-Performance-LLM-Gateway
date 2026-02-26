# Go Development Patterns - Skill File

## Overview
This skill captures essential Go (Golang) patterns, best practices, and conventions for building robust, efficient, and maintainable applications.

## When to Activate
- Writing new Go code
- Reviewing Go code
- Refactoring existing Go code
- Designing Go packages/modules

## Core Principles

**Simplicity Over Cleverness**
Go values clarity. Code should be obvious and readable. Avoid overly clever one-liners that sacrifice readability.

**Make Zero Values Useful**
Design types so their zero values work immediately without initialization. For example, `sync.Mutex` and `Counter` work with zero values, but `map[string]int` nil maps will panic.

**Accept Interfaces, Return Structs**
Functions should accept interface parameters and return concrete types. This provides flexibility while hiding implementation details.

## Error Handling Patterns

**Wrap Errors with Context**
Use `fmt.Errorf` with `%w` to preserve the original error while adding context.

**Define Custom Error Types**
Create domain-specific errors that implement the `Error()` interface. Use sentinel errors for common cases.

**Check Errors Properly**
Use `errors.Is()` to check for specific errors and `errors.As()` to check error types.

**Never Ignore Errors**
Handle errors explicitly or document why it's safe to ignore. Avoid using `_` to discard errors.

## Concurrency Patterns

**Worker Pool**
Use goroutines with channels to distribute work across multiple workers, coordinating with `sync.WaitGroup`.

**Context for Cancellation and Timeout**
Use `context.WithTimeout` and `context.WithCancel` to manage request scope and prevent resource leaks.

**Graceful Shutdown**
Listen for OS signals (SIGINT, SIGTERM) and use `http.Server.Shutdown()` with a timeout for clean server shutdown.

**errgroup for Goroutine Coordination**
Use `golang.org/x/sync/errgroup` to manage multiple goroutines with shared context and error collection.

**Prevent Goroutine Leaks**
Use buffered channels and select statements with `ctx.Done()` to handle cancellation properly.

## Interface Design

**Small, Focused Interfaces**
Prefer single-method interfaces like `io.Reader` and `io.Writer`. Compose them as needed.

**Define Interfaces Where Used**
Create interfaces in the consumer package, not the provider. The implementation doesn't need to know about the interface.

**Optional Behavior with Type Assertions**
Check for optional interface implementations using type assertions when behavior is optional.

## Package Organization

**Standard Project Layout**
```
myproject/
├── cmd/myapp/        # Entry point
├── internal/         # Private code (handler, service, repository)
├── pkg/              # Public code
├── api/              # API definitions
└── testdata/         # Test fixtures
```

**Package Naming**
Use short, lowercase names without underscores: `http`, `json`, `user` (not `userService`).

**Avoid Package-Level State**
Use dependency injection rather than global variables. Pass dependencies as struct fields.

## Struct Design

**Functional Options Pattern**
Use function options for flexible parameters.

**Embedding constructor with many optional for Composition**
Embed types to inherit methods, but be explicit about the relationship.

## Memory and Performance

**Pre-allocate Slices**
Use `make([]Type, 0, len(items))` when the final size is known to avoid multiple reallocations.

**Use sync.Pool for Frequent Allocations**
Pool reusable objects like buffers to reduce garbage collection pressure.

**Avoid String Concatenation in Loops**
Use `strings.Builder` or `strings.Join` instead of `+=` in loops.

## Go Tools Integration

**Essential Commands**
```bash
go build ./...
go test -race ./...
go vet ./...
go mod tidy
gofmt -w .
```

**Recommended Linters**
Enable: errcheck, govet, staticcheck, unused, gofmt, goimports, ineffassign

## Key Idioms Summary

| Idiom | Description |
|-------|-------------|
| Accept interfaces, return structs | Flexibility with implementation hiding |
| Errors are values | Treat errors as first-class return values |
| Don't communicate through shared memory | Use channels for goroutine coordination |
| Make zero values useful | Types should work without initialization |
| Clarity over cleverness | Prioritize readability |
| Early returns | Handle errors first, keep main path unindented |
| gofmt is everyone's friend | Always format code consistently |

## Anti-Patterns to Avoid

- Naked returns in long functions
- Using panic for control flow
- Passing context as struct field instead of first parameter
- Mixing value and pointer receivers inconsistently
- Ignoring errors with `_`

## Final Principle
Go code should feel "boring" in the best way—predictable, consistent, and easy to understand. When in doubt, keep it simple.
