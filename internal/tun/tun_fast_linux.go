//go:build linux

package tun

import (
	"os"
	"reflect"

	"github.com/songgao/water"
	"seras-protocol/internal/iouring"
)

// FastTUN wraps TUN with io_uring for async I/O on Linux
type FastTUN struct {
	*TUN
	ring iouring.Ring
	fd   int
}

// NewFast creates a TUN with io_uring acceleration
func NewFast(localIP, gateway, nodeIP, nodeVPNIP string) (*FastTUN, error) {
	return NewFastWithDNS(localIP, gateway, nodeIP, nodeVPNIP, []string{"8.8.8.8", "1.1.1.1"})
}

// NewFastWithDNS creates a TUN with io_uring and custom DNS
func NewFastWithDNS(localIP, gateway, nodeIP, nodeVPNIP string, dnsServers []string) (*FastTUN, error) {
	t, err := NewWithDNS(localIP, gateway, nodeIP, nodeVPNIP, dnsServers)
	if err != nil {
		return nil, err
	}

	ft := &FastTUN{TUN: t, fd: -1}

	// Try to enable io_uring
	if iouring.IsSupported() {
		fd := extractFD(t.dev)
		if fd >= 0 {
			ring, err := iouring.New(iouring.DefaultConfig())
			if err == nil {
				ft.ring = ring
				ft.fd = fd
			}
		}
	}

	return ft, nil
}

// NewFastNode creates a node TUN with io_uring
func NewFastNode(localIP, vpnSubnet string) (*FastTUN, error) {
	t, err := NewNodeTUN(localIP, vpnSubnet)
	if err != nil {
		return nil, err
	}

	ft := &FastTUN{TUN: t, fd: -1}

	if iouring.IsSupported() {
		fd := extractFD(t.dev)
		if fd >= 0 {
			ring, err := iouring.New(iouring.DefaultConfig())
			if err == nil {
				ft.ring = ring
				ft.fd = fd
			}
		}
	}

	return ft, nil
}

// ReadAsync performs async read using io_uring
func (t *FastTUN) ReadAsync(buf []byte) (iouring.AsyncOp, error) {
	if t.ring != nil && t.fd >= 0 {
		return t.ring.ReadAsync(t.fd, buf)
	}
	// Fallback to blocking
	n, err := t.TUN.Read(buf)
	return &immediateOp{n: n, err: err}, nil
}

// WriteAsync performs async write using io_uring
func (t *FastTUN) WriteAsync(buf []byte) (iouring.AsyncOp, error) {
	if t.ring != nil && t.fd >= 0 {
		return t.ring.WriteAsync(t.fd, buf)
	}
	// Fallback to blocking
	n, err := t.TUN.Write(buf)
	return &immediateOp{n: n, err: err}, nil
}

// HasIOURing returns true if io_uring is active
func (t *FastTUN) HasIOURing() bool {
	return t.ring != nil && t.fd >= 0
}

func (t *FastTUN) Close() error {
	if t.ring != nil {
		t.ring.Close()
	}
	return t.TUN.Close()
}

// extractFD gets the file descriptor from water.Interface
func extractFD(dev *water.Interface) int {
	// Use reflection to get the underlying file descriptor
	v := reflect.ValueOf(dev).Elem()
	rwc := v.FieldByName("ReadWriteCloser")
	if !rwc.IsValid() {
		return -1
	}

	// Try to get *os.File
	if f, ok := rwc.Interface().(*os.File); ok {
		return int(f.Fd())
	}

	return -1
}

// immediateOp is a completed operation (for fallback)
type immediateOp struct {
	n   int
	err error
}

func (o *immediateOp) Wait() (int, error) {
	return o.n, o.err
}

func (o *immediateOp) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
