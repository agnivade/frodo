# frodo
Demo API to play with io_uring in Go

1. Install liburing in your machine from latest master. (https://github.com/axboe/liburing/)
2. Download the Go toolchain (https://golang.org/dl/)
3. https://golang.org/doc/install#tarball
4. Unpack the tarball into any directory.
5. Do "go build" to ensure library builds fine.
6. Now at the parent directory of frodo, create another directory to test the library.
7. Go inside that.
8. go mod init name_of_the_directory
9. Edit the go.mod file to look like this:
```
module name_of_directory

go 1.14

require github.com/agnivade/frodo v0.0.0-20200412054840-178d9973f200 // indirect

replace github.com/agnivade/frodo => ../frodo
```

10. Create another file main.go like this:
```
package main
 
import (
  "fmt"
  "time"

  "github.com/agnivade/frodo"
)

func main() {
  frodo.Init()
  defer frodo.Cleanup()

  err := frodo.Hello("path/to/any/file")
  if err != nil {
    fmt.Println(err)
  }

  time.Sleep(1 * time.Second)
}
```

11. go run main.go

This should print the file in the console.
