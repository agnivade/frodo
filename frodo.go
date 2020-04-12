package frodo

/*
#cgo LDFLAGS: -luring
#include <fcntl.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/ioctl.h>
#include <liburing.h>
#include <stdlib.h>

extern off_t get_file_size(int);
extern int get_completion_and_print(struct io_uring *);
extern int submit_read_request(int, off_t, struct io_uring *);
extern int cat_file(int, off_t);
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
)

//export printToConsole
func printToConsole(cstr *C.char) {
	str := C.GoString(cstr)
	fmt.Println(str)
}

func Hello(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	size := fi.Size()
	fd := f.Fd()
	ret := int(C.cat_file(C.int(fd), C.long(size)))
	if ret != 0 {
		return errors.New("non-zero exit code")
	}
	return nil
}

// func ff() {
// 	// queue is already inited
// 	for {
// 		select {
// 		case sqe := <-submitChan:
// 			// io_uring_get_sqe
// 			// io_uring_prep readv/writev
// 			// io_uring_sqe_set_data

// 			// if let's say more than some threshold, then
// 			// io_uring_submit
// 			for i := 0; i < threshold; i++ {
// 				// io_uring_wait_cqe
// 				// io_uring_cqe_get_data
// 				// io_uring_cqe_seen;
// 			}
// 		case <-ticker.C: // some timer of some interval
// 			// if items in queue, then
// 			// io_uring_submit
// 			for i := 0; i < remaining_items; i++ {
// 				// io_uring_wait_cqe
// 				// io_uring_cqe_get_data
// 				// io_uring_cqe_seen;
// 			}
// 		}
// 	}
// }
