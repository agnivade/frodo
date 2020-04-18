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
extern int queue_submit(int);
extern void queue_exit();
*/
import "C"

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"
)

type opCode int

const (
	opCodeRead opCode = iota + 1
	opCodeWrite
)

const EAGAIN = -11

type request struct {
	code opCode
	fd   uintptr
	size int64
}

type cbInfo struct {
	cb    func([]byte)
	close func() error
}

var (
	quitChan   chan struct{}
	submitChan chan *request
	cbMut      sync.RWMutex
	cbMap      map[uintptr]cbInfo
)

//export read_callback
func read_callback(iovecs *C.struct_iovec, length C.int, fd C.int) {
	// Here be dragons.
	// defer C.free(unsafe.Pointer(iovecs)) // This makes it crash.
	intLen := int(length)
	slice := (*[1 << 28]C.struct_iovec)(unsafe.Pointer(iovecs))[:intLen:intLen]
	// Can be optimized further with more unsafe.
	var buf bytes.Buffer
	for i := 0; i < intLen; i++ {
		_, err := buf.Write(C.GoBytes(slice[i].iov_base, C.int(slice[i].iov_len)))
		if err != nil {
			fmt.Println("err while writing", err)
		}
	}
	cbMut.RLock()
	cbMap[uintptr(fd)].close()
	cbMap[uintptr(fd)].cb(buf.Bytes())
	cbMut.RUnlock()
}

func Init() error {
	ret := int(C.queue_init())
	if ret < 0 {
		return fmt.Errorf("queue init failed with %d exit code", ret)
	}
	quitChan = make(chan struct{})
	submitChan = make(chan *request)
	cbMap = make(map[uintptr]cbInfo)
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
				fmt.Printf("non-zero return code while pushing: %d\n", ret)
				continue
			}

			queueSize++
			if queueSize > 5 { // if queue_size > threshold, then pop all.
				// TODO: maybe just pop one
				ret = int(C.queue_submit(C.int(queueSize)))
				if ret < 0 {
					fmt.Printf("non-zero return code while submitting: %d\n", ret)
					return
				}
				for queueSize > 0 {
					ret = int(C.pop_request())
					if ret != 0 {
						fmt.Printf("non-zero return code while popping: %d\n", ret)
						if ret != EAGAIN { // Do not decrement if nothing was read.
							queueSize--
						}
						continue
					}
					queueSize--
				}
			}
		case <-ticker.C: // some timer of some interval
			if queueSize > 0 {
				ret := int(C.queue_submit(C.int(queueSize)))
				if ret < 0 {
					fmt.Printf("non-zero return code while submitting: %d\n", ret)
					return
				}
				for queueSize > 0 {
					ret := int(C.pop_request())
					if ret != 0 {
						fmt.Printf("non-zero return code while popping: %d\n", ret)
						if ret != EAGAIN { // Do not decrement if nothing was read.
							queueSize--
						}
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

func ReadFile(path string, cb func(buf []byte)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	cbMut.Lock()
	cbMap[f.Fd()] = cbInfo{
		cb:    cb,
		close: f.Close,
	}
	cbMut.Unlock()

	submitChan <- &request{
		code: opCodeRead,
		fd:   f.Fd(),
		size: fi.Size(),
	}
	return nil
}
