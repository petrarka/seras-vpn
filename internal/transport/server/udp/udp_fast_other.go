//go:build !linux

package udp

import "fmt"

// FastServer is not available on non-Linux
type FastServer struct{}

// NewFastServer returns error on non-Linux
func NewFastServer(addr string, onMessage func(conn *Connection, data []byte)) (*FastServer, error) {
	return nil, fmt.Errorf("io_uring is only available on Linux")
}

// SetOnDisconnect is a no-op
func (s *FastServer) SetOnDisconnect(callback func(conn *Connection)) {}

// Start returns error
func (s *FastServer) Start() error {
	return fmt.Errorf("io_uring is only available on Linux")
}

// Stop is a no-op
func (s *FastServer) Stop() error {
	return nil
}

// IsFastSupported returns false on non-Linux
func IsFastSupported() bool {
	return false
}
