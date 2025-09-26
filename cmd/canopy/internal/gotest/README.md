# gotest Package

The `gotest` package wraps Go's native test output for use within canopy.

## Components

- **definition.go** - AST-based discovery of test functions and embedded t.Run cases
- **reference.go** - Hierarchical test identification (package/function/subtest)
- **runner.go** - Test execution coordination
- **event.go** - Test execution events and state tracking
- **result.go** - Aggregated test results with concurrent access
- **jsonl.go** - Parsing of `go test -json` output
- **selector.go** - Test selection logic

## Usage

### Running Tests
```go
runner := NewRunner(config)
result, err := runner.Run(ctx, references)
```

### Processing Events
```go
runner.OnEvent(func(event Event) {
    // Handle test progress
})
```

### Querying Results
```go
passed := result.ReferencesByAction(ActionPass)
coverage := result.Coverage()
```

