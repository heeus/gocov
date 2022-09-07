# gocov

Originally github.com/dave/courtney

Allows to exclude some fragments of code from go test coverage.

# Excludes 
What do we exclude from the coverage report?

### Blocks including a panic 
If you need to test that your code panics correctly, it should probably be an 
error rather than a panic. 

### notest comments
Blocks or files with a `// notest` comment are excluded.

### Blocks returning a error tested to be non-nil
We only exclude blocks where the error being returned has been tested to be 
non-nil, so:

```go
err := foo()
if err != nil {
    return err // excluded 
}
```

... however:

```go
if i == 0 {
    return errors.New("...") // not excluded
}
```

A few more rules:
* If multiple return values are returned, error must be the last, and all 
others must be nil or zero values.  
* We also exclude blocks returning an error which is the result of a function 
taking a non-nil error as a parameter, e.g. `errors.Wrap(err, "...")`.  
* We also exclude blocks containing a bare return statement, where the function 
has named result parameters, and the last result is an error that has been 
tested non-nil. Be aware that in this scenario no attempt is made to verify 
that the other result parameters are zero values.  

# Usage
Run the gocov command followed by a list of packages. Use `.` for the 
package in the current directory, and adding `/...` tests all sub-packages 
recursively. If no packages are provided, the default is `./...`.

## Quick start: 

### Install
```
go install github.com/heeus/gocov@latest 
```
### Run: 

```
gocov
gocov .
gocov ./...

gocov -uncover
gocov -notest
```

### Output: 

```
The following lines are not tested:
core-istructsmem/resources-types.go:77:88
core-istructsmem/resources-types.go:233:83
core-istructsmem/viewrecords-dynobuf.go:184:10
core-istructsmem/viewrecords-dynobuf.go:200:10
coverage: 99.8% of statements
```
Each line like

```common
core-istructsmem/resources-types.go:77:88
```
will be treated by VSCode as a link to source file.

### Verbose: -v
`Verbose output`
All the output from the `go test -v` command is shown.

