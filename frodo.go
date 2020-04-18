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

extern int queue_init();
extern int push_request(int, off_t);
extern int pop_request();
extern int queue_submit();
extern void queue_exit();
*/
import "C"

import (
	"fmt"
	"os"
	"time"
)

type opCode int

const (
	opCodeRead opCode = iota + 1
	opCodeWrite
)

type request struct {
	code opCode
	fd   uintptr
	size int64
}

var (
	quitChan   chan struct{}
	submitChan chan *request
)

//export printToConsole
func printToConsole(cstr *C.char) {
	str := C.GoString(cstr)
	fmt.Println(str)
}

func Init() error {
	ret := int(C.queue_init())
	if ret < 0 {
		return fmt.Errorf("queue init failed with %d exit code", ret)
	}
	quitChan = make(chan struct{})
	submitChan = make(chan *request)
	go startLoop()
	return nil
}

func Cleanup() {
	quitChan <- struct{}{}
	C.queue_exit()
}

func startLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	queueSize := 0
	for {
		select {
		case sqe := <-submitChan:
			ret := int(C.push_request(C.int(sqe.fd), C.long(sqe.size)))
			if ret < 0 {
				fmt.Printf("non-zero return code: %d\n", ret)
				continue
			}

			queueSize++
			if queueSize > 5 { // if queue_size == queue_depth, then submit and pop 1.
				ret = int(C.queue_submit())
				if ret < 0 {
					fmt.Printf("non-zero return code: %d\n", ret)
					return
				}
				for queueSize > 0 {
					ret = int(C.pop_request())
					if ret != 0 {
						fmt.Printf("non-zero return code: %d\n", ret)
						queueSize--
						continue
					}
					queueSize--
				}
			}
		case <-ticker.C: // some timer of some interval
			if queueSize > 0 {
				ret := int(C.queue_submit())
				if ret < 0 {
					fmt.Printf("non-zero return code: %d\n", ret)
					return
				}
				for queueSize > 0 {
					ret := int(C.pop_request())
					if ret != 0 {
						fmt.Printf("non-zero return code: %d\n", ret)
						queueSize--
						continue
					}
					queueSize--
				}
			}
		case <-quitChan:
			// possibly drain channel.
			// pop_request till everything is done.
			return
		}
	}
}

func Hello(path string) (error, func() error) {
	f, err := os.Open(path)
	if err != nil {
		return err, nil
	}

	fi, err := f.Stat()
	if err != nil {
		return err, f.Close
	}

	submitChan <- &request{
		code: opCodeRead,
		fd:   f.Fd(),
		size: fi.Size(),
	}
	return nil, f.Close
}
