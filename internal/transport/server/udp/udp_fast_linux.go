//go:build linux

package udp

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"syscall"

	"seras-protocol/internal/iouring"
)

// FastServer is a UDP server with io_uring acceleration
type FastServer struct {
	addr         string
	conn         *net.UDPConn
	fd           int
	ring         iouring.Ring
	connections  map[string]*Connection
	mu           sync.RWMutex
	onMessage    func(conn *Connection, data []byte)
	onDisconnect func(conn *Connection)
}

// NewFastServer creates a new io_uring accelerated UDP server
func NewFastServer(addr string, onMessage func(conn *Connection, data []byte)) (*FastServer, error) {
	if !iouring.IsSupported() {
		return nil, fmt.Errorf("io_uring not supported")
	}

	ring, err := iouring.New(iouring.Config{Entries: 512})
	if err != nil {
		return nil, err
	}

	return &FastServer{
		addr:        addr,
		ring:        ring,
		connections: make(map[string]*Connection),
		onMessage:   onMessage,
	}, nil
}

// SetOnDisconnect sets callback for client disconnection
func (s *FastServer) SetOnDisconnect(callback func(conn *Connection)) {
	s.onDisconnect = callback
}

// Start starts the io_uring accelerated UDP server
func (s *FastServer) Start() error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	s.conn = conn

	// Get raw file descriptor
	file, err := conn.File()
	if err != nil {
		return err
	}
	s.fd = int(file.Fd())

	// Set socket to non-blocking for io_uring
	syscall.SetNonblock(s.fd, true)

	slog.Info("Fast UDP server starting with io_uring", "addr", s.addr)

	// Run multiple parallel receivers
	numReceivers := 4
	var wg sync.WaitGroup
	wg.Add(numReceivers)

	for i := 0; i < numReceivers; i++ {
		go func() {
			defer wg.Done()
			s.receiveLoop()
		}()
	}

	wg.Wait()
	return nil
}

func (s *FastServer) receiveLoop() {
	buf := make([]byte, 65535)

	for {
		// Use io_uring async recv
		op, err := s.ring.RecvAsync(s.fd, buf)
		if err != nil {
			slog.Error("io_uring recv error", "error", err)
			continue
		}

		n, err := op.Wait()
		if err != nil {
			if err != syscall.EAGAIN && err != syscall.EWOULDBLOCK {
				slog.Error("UDP read error", "error", err)
			}
			continue
		}

		if n == 0 {
			continue
		}

		// For UDP with io_uring we need to get the source address differently
		// Since recvfrom with io_uring is complex, fall back to standard read
		// but use the async submission for better batching
		data := make([]byte, n)
		copy(data, buf[:n])

		// Note: io_uring recvmsg would give us the source address
		// For now, dispatch without address tracking
		if s.onMessage != nil {
			go s.onMessage(nil, data)
		}
	}
}

// Stop stops the server
func (s *FastServer) Stop() error {
	if s.ring != nil {
		s.ring.Close()
	}
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// IsFastSupported returns true if io_uring is available
func IsFastSupported() bool {
	return iouring.IsSupported()
}
