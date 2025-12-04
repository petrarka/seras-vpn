//go:build !linux

package iouring

import (
	"errors"
	"syscall"
)

var ErrNotSupported = errors.New("io_uring is only supported on Linux")

type fallbackRing struct{}

type fallbackAsyncOp struct {
	n    int
	err  error
	done chan struct{}
}

// New returns an error on non-Linux systems
func New(cfg Config) (Ring, error) {
	return nil, ErrNotSupported
}

// IsSupported returns false on non-Linux systems
func IsSupported() bool {
	return false
}

// NewFallback creates a fallback ring that uses blocking I/O
// This allows code to work on all platforms, just without io_uring benefits
func NewFallback() Ring {
	return &fallbackRing{}
}

func (r *fallbackRing) ReadAsync(fd int, buf []byte) (AsyncOp, error) {
	op := &fallbackAsyncOp{done: make(chan struct{})}
	go func() {
		defer close(op.done)
		op.n, op.err = syscall.Read(fd, buf)
	}()
	return op, nil
}

func (r *fallbackRing) WriteAsync(fd int, buf []byte) (AsyncOp, error) {
	op := &fallbackAsyncOp{done: make(chan struct{})}
	go func() {
		defer close(op.done)
		op.n, op.err = syscall.Write(fd, buf)
	}()
	return op, nil
}

func (r *fallbackRing) RecvAsync(fd int, buf []byte) (AsyncOp, error) {
	op := &fallbackAsyncOp{done: make(chan struct{})}
	go func() {
		defer close(op.done)
		op.n, _, op.err = syscall.Recvfrom(fd, buf, 0)
	}()
	return op, nil
}

func (r *fallbackRing) SendAsync(fd int, buf []byte) (AsyncOp, error) {
	op := &fallbackAsyncOp{done: make(chan struct{})}
	go func() {
		defer close(op.done)
		op.err = syscall.Sendto(fd, buf, 0, nil)
		if op.err == nil {
			op.n = len(buf)
		}
	}()
	return op, nil
}

func (r *fallbackRing) Submit() error {
	return nil
}

func (r *fallbackRing) Close() error {
	return nil
}

func (op *fallbackAsyncOp) Wait() (int, error) {
	<-op.done
	return op.n, op.err
}

func (op *fallbackAsyncOp) Done() <-chan struct{} {
	return op.done
}
