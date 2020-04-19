package frodo_test

import (
	"fmt"
	"sync"

	"github.com/agnivade/frodo"
)

func Example() {
	err := frodo.Init()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer frodo.Cleanup()

	go func() {
		for err := range frodo.Err() {
			fmt.Println(err)
		}
	}()

	var wg sync.WaitGroup
	// Read a file.
	err = frodo.ReadFile("testdata/ssa.html", func(buf []byte) {
		defer wg.Done()
		// handle buf
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	// Write something
	err = frodo.WriteFile("testdata/dummy.txt", []byte("hello world"), 0644, func(n int) {
		defer wg.Done()
		// handle n
	})

	// Call Poll to let the kernel know to read the entries.
	frodo.Poll()
	// Wait till all callbacks are done.
	wg.Wait()
}
