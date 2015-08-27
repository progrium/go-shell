# go-shell

Library to write "shelling out" Go code more shell-like,
while remaining idiomatic to Go.

## Features

 * Function-wrapper factories for shell commands
 * Panic on non-zero exits for `set -e` behavior
 * Result of `Run()` is a Stringer for STDOUT, has Error for STDERR
 * Heavily variadic function API `Cmd("rm", "-r", "foo") == Cmd("rm -r", "foo")`
 * Go-native piping `Cmd(...).Pipe(...)` or inline piping `Cmd("... | ...")`
 * Template compatible "last arg" piping `Cmd(..., Cmd(..., Cmd(...)))`
 * Optional trace output mode like `set +x`
 * Similar variadic functions for paths and path templates

## Examples

```go
import (
  "fmt"
  "github.com/progrium/go-shell"
)

var (
  sh = shell.Run
)

shell.Trace = true // like set +x
shell.Shell = []string{"/bin/bash", "-c"} // defaults to /bin/sh

func main() {
  defer shell.ErrExit()
  sh("echo Foobar > /foobar")
  sh("rm /fobar") // typo raises error
  sh("echo Done!") // never run, program exited
}
```

```go
import (
  "fmt"
  "github.com/progrium/go-shell"
)

func main() {
  fmt.Println(shell.Cmd("echo", "foobar").Pipe("wc", "-c").Pipe("awk", "'{print $1}'").Run())
}
```

```go
import "github.com/progrium/go-shell"

var (
  echo = shell.Cmd("echo").OutputFn()
  copy = shell.Cmd("cp").ErrFn()
  rm = shell.Cmd("rm").ErrFn()
)

func main() {
  err := copy("/foo", "/bar")
  // handle err
  err = rm("/bar")
  // handle err
  out, _ := echo("Done!")
}
```

## License

MIT
