package iouring

// Ring is the interface for async I/O operations
type Ring interface {
	// ReadAsync queues an async read operation
	ReadAsync(fd int, buf []byte) (AsyncOp, error)
	// WriteAsync queues an async write operation
	WriteAsync(fd int, buf []byte) (AsyncOp, error)
	// RecvAsync queues an async recv operation (for sockets)
	RecvAsync(fd int, buf []byte) (AsyncOp, error)
	// SendAsync queues an async send operation (for sockets)
	SendAsync(fd int, buf []byte) (AsyncOp, error)
	// Submit submits all queued operations
	Submit() error
	// Close closes the ring
	Close() error
}

// AsyncOp represents an async operation result
type AsyncOp interface {
	// Wait waits for the operation to complete and returns bytes processed
	Wait() (int, error)
	// Done returns a channel that's closed when operation completes
	Done() <-chan struct{}
}

// Config for io_uring
type Config struct {
	Entries    uint32 // Queue depth (default 256)
	BufferSize int    // Buffer size for operations
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		Entries:    256,
		BufferSize: 65536,
	}
}
