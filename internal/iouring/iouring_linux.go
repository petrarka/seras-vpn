//go:build linux

package iouring

import (
	"fmt"
	"sync"
	"syscall"

	"github.com/iceber/iouring-go"
)

type linuxRing struct {
	ring *iouring.IOURing
	mu   sync.Mutex
}

type linuxAsyncOp struct {
	request iouring.Request
	done    chan struct{}
	n       int
	err     error
}

// New creates a new io_uring ring on Linux
func New(cfg Config) (Ring, error) {
	if cfg.Entries == 0 {
		cfg.Entries = 256
	}

	ring, err := iouring.New(uint(cfg.Entries))
	if err != nil {
		return nil, fmt.Errorf("failed to create io_uring: %w", err)
	}

	return &linuxRing{ring: ring}, nil
}

// IsSupported returns true on Linux with kernel >= 5.1
func IsSupported() bool {
	ring, err := iouring.New(1)
	if err != nil {
		return false
	}
	ring.Close()
	return true
}

func (r *linuxRing) ReadAsync(fd int, buf []byte) (AsyncOp, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	op := &linuxAsyncOp{done: make(chan struct{})}

	prep := iouring.Read(fd, buf)
	request, err := r.ring.SubmitRequest(prep, nil)
	if err != nil {
		return nil, err
	}

	op.request = request
	go op.waitForCompletion()
	return op, nil
}

func (r *linuxRing) WriteAsync(fd int, buf []byte) (AsyncOp, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	op := &linuxAsyncOp{done: make(chan struct{})}

	prep := iouring.Write(fd, buf)
	request, err := r.ring.SubmitRequest(prep, nil)
	if err != nil {
		return nil, err
	}

	op.request = request
	go op.waitForCompletion()
	return op, nil
}

func (r *linuxRing) RecvAsync(fd int, buf []byte) (AsyncOp, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	op := &linuxAsyncOp{done: make(chan struct{})}

	prep := iouring.Recv(fd, buf, 0)
	request, err := r.ring.SubmitRequest(prep, nil)
	if err != nil {
		return nil, err
	}

	op.request = request
	go op.waitForCompletion()
	return op, nil
}

func (r *linuxRing) SendAsync(fd int, buf []byte) (AsyncOp, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	op := &linuxAsyncOp{done: make(chan struct{})}

	prep := iouring.Send(fd, buf, 0)
	request, err := r.ring.SubmitRequest(prep, nil)
	if err != nil {
		return nil, err
	}

	op.request = request
	go op.waitForCompletion()
	return op, nil
}

func (r *linuxRing) Submit() error {
	return nil // auto-submitted in our implementation
}

func (r *linuxRing) Close() error {
	return r.ring.Close()
}

func (op *linuxAsyncOp) waitForCompletion() {
	defer close(op.done)
	<-op.request.Done()
	n, err := op.request.GetRes()
	if err != nil {
		op.err = err
		return
	}
	if n < 0 {
		op.err = syscall.Errno(-n)
		return
	}
	op.n = n
}

func (op *linuxAsyncOp) Wait() (int, error) {
	<-op.done
	return op.n, op.err
}

func (op *linuxAsyncOp) Done() <-chan struct{} {
	return op.done
}
