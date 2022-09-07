# gocov

Exclude some code from test coverage reports

- Originally github.com/dave/courtney
- Main differences
  - `gocov` does NOT implicitly exclude any code
  - `gocov` prints not-covered lines in a format which is understandable by VS Code:
```go
The following lines are not tested:
./context.go:47:2
./context.go:102:101
./ebnf.go:105:18
./ebnf.go:114:17
./ebnf.go:118:19
./ebnf/ebnf.go:41:26
```
# Quick start: 

- Install: `go install github.com/heeus/gocov@latest`
- Exclude code from test coverage:
  - Exclude the rest of the code block forever: `// notest`
  - Exclude the rest of the code block due to lack of time: `// notestdept`
- Show coverage-excluded code:
    - Excluded by // notest: `gocov notest`
    - Excluded by // notestdept : `gocov notestdept`
- Run tests and show uncovered lines:
  - Current package: `gocov .`
  - Current package + sub-packages: `gocov ./...`
  - Default (if nothing specified): `.`
- Verbose mode
  - Show output from the `go test -v`: `gocov -v`

# Exclusion nuances

```go
func foo() {
  fmt.Println("foo 1")
  fmt.Println("foo 2")
  // notest
  fmt.Println("foo 3") // excluded
  fmt.Println("foo 4") // excluded
}
```

```go
func foo() {  
  fmt.Println("foo 1")
  fmt.Println("foo 2")
  {
    // notest
    fmt.Println("foo 3") // excluded
  }
  fmt.Println("foo 4")
}
```