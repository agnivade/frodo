package frodo

/*
#cgo LDFLAGS: -luring
#include <fcntl.h>
#include <liburing.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/stat.h>

extern int queue_init();
extern int push_read_request(int, off_t);
extern int push_write_request(int, void *, off_t);
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
	"syscall"
	"unsafe"
)

type opCode int

const (
	opCodeRead opCode = iota + 1
	opCodeWrite
)

const EAGAIN = -11
const queueThreshold = 5

type readCallback func([]byte)
type writeCallback func(int)

type request struct {
	code    opCode
	f       *os.File
	buf     []byte
	size    int64
	readCb  readCallback
	writeCb writeCallback
}

type cbInfo struct {
	readCb  readCallback
	writeCb writeCallback
	close   func() error
}

// TODO: move to local struct fields.
var (
	globalMut  sync.RWMutex
	quitChan   chan struct{}
	submitChan chan *request
	pollChan   chan struct{}
	errChan    chan error

	cbMut sync.RWMutex
	cbMap map[uintptr]cbInfo
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
			errChan <- fmt.Errorf("error during buffer write: %v", err)
		}
	}
	cbMut.RLock()
	cbMap[uintptr(fd)].close()
	cbMap[uintptr(fd)].readCb(buf.Bytes())
	cbMut.RUnlock()
}

//export write_callback
func write_callback(written C.int, fd C.int) {
	cbMut.RLock()
	cbMap[uintptr(fd)].close()
	cbMap[uintptr(fd)].writeCb(int(written))
	cbMut.RUnlock()
}

func Init() error {
	ret := int(C.queue_init())
	if ret < 0 {
		return fmt.Errorf("%v", syscall.Errno(-ret))
	}
	globalMut.Lock()
	quitChan = make(chan struct{})
	pollChan = make(chan struct{})
	errChan = make(chan error)
	submitChan = make(chan *request)
	cbMap = make(map[uintptr]cbInfo)
	globalMut.Unlock()
	go startLoop()
	return nil
}

func Cleanup() {
	quitChan <- struct{}{}
	C.queue_exit()
	close(submitChan)
	close(errChan)
}

func Err() <-chan error {
	globalMut.RLock()
	defer globalMut.RUnlock()
	return errChan
}

func startLoop() {
	queueSize := 0
	for {
		select {
		case sqe := <-submitChan:
			switch sqe.code {
			case opCodeRead:
				// No need for locking here.
				cbMap[sqe.f.Fd()] = cbInfo{
					readCb: sqe.readCb,
					close:  sqe.f.Close,
				}

				ret := int(C.push_read_request(C.int(sqe.f.Fd()), C.long(sqe.size)))
				if ret < 0 {
					errChan <- fmt.Errorf("error while pushing read request: %v", syscall.Errno(-ret))
					continue
				}
			case opCodeWrite:
				// No need for locking here.
				cbMap[sqe.f.Fd()] = cbInfo{
					writeCb: sqe.writeCb,
					close:   sqe.f.Close,
				}

				var ptr unsafe.Pointer
				if len(sqe.buf) == 0 {
					zeroBytes := []byte("")
					ptr = unsafe.Pointer(&zeroBytes)
				} else {
					ptr = unsafe.Pointer(&sqe.buf[0])
				}

				ret := int(C.push_write_request(C.int(sqe.f.Fd()), ptr, C.long(len(sqe.buf))))
				if ret < 0 {
					errChan <- fmt.Errorf("error while pushing write_request: %v", syscall.Errno(-ret))
					continue
				}
			}

			queueSize++
			if queueSize > queueThreshold { // if queue_size > threshold, then pop all.
				// TODO: maybe just pop one
				submitAndPop(queueSize)
				queueSize = 0
			}
		case <-pollChan:
			if queueSize > 0 {
				submitAndPop(queueSize)
				queueSize = 0
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

	submitChan <- &request{
		code:   opCodeRead,
		f:      f,
		size:   fi.Size(),
		readCb: cb,
	}
	return nil
}

func WriteFile(path string, data []byte, perm os.FileMode, cb func(written int)) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	submitChan <- &request{
		code:    opCodeWrite,
		buf:     data,
		f:       f,
		writeCb: cb,
	}
	return nil
}

func Poll() {
	// TODO: do we allow user to set wait_nr ?
	pollChan <- struct{}{}
}

func submitAndPop(queueSize int) {
	ret := int(C.queue_submit(C.int(queueSize)))
	if ret < 0 {
		errChan <- fmt.Errorf("error while submitting: %v", syscall.Errno(-ret))
		return
	}
	for queueSize > 0 {
		ret := int(C.pop_request())
		if ret != 0 {
			errChan <- fmt.Errorf("error while popping: %v", syscall.Errno(-ret))
			if syscall.Errno(-ret) != syscall.EAGAIN { // Do not decrement if nothing was read.
				queueSize--
			}
			continue
		}
		queueSize--
	}
}
