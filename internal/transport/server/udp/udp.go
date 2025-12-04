package udp

import (
	"log/slog"
	"net"
	"sync"
)

// Connection represents a UDP client identified by address
type Connection struct {
	addr   *net.UDPAddr
	server *Server
}

// Send sends data to this client
func (c *Connection) Send(data []byte) error {
	_, err := c.server.conn.WriteToUDP(data, c.addr)
	return err
}

// Server is a UDP server for node
type Server struct {
	addr         string
	conn         *net.UDPConn
	connections  map[string]*Connection // key is addr.String()
	mu           sync.RWMutex
	onMessage    func(conn *Connection, data []byte)
	onDisconnect func(conn *Connection)
}

// NewServer creates a new UDP server
func NewServer(addr string, onMessage func(conn *Connection, data []byte)) *Server {
	return &Server{
		addr:        addr,
		connections: make(map[string]*Connection),
		onMessage:   onMessage,
	}
}

// SetOnDisconnect sets callback for client disconnection
func (s *Server) SetOnDisconnect(callback func(conn *Connection)) {
	s.onDisconnect = callback
}

// Start starts the UDP server
func (s *Server) Start() error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	s.conn = conn

	slog.Info("UDP server starting", "addr", s.addr)

	buf := make([]byte, 65535)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			slog.Error("UDP read error", "error", err)
			continue
		}

		// Get or create connection for this client
		addrKey := clientAddr.String()
		s.mu.Lock()
		clientConn, exists := s.connections[addrKey]
		if !exists {
			clientConn = &Connection{
				addr:   clientAddr,
				server: s,
			}
			s.connections[addrKey] = clientConn
			slog.Info("New UDP client", "addr", addrKey)
		}
		s.mu.Unlock()

		// Copy data and dispatch
		data := make([]byte, n)
		copy(data, buf[:n])

		if s.onMessage != nil {
			go s.onMessage(clientConn, data)
		}
	}
}

// RemoveConnection removes a client connection
func (s *Server) RemoveConnection(conn *Connection) {
	s.mu.Lock()
	delete(s.connections, conn.addr.String())
	s.mu.Unlock()

	if s.onDisconnect != nil {
		s.onDisconnect(conn)
	}
	slog.Info("UDP client removed", "addr", conn.addr.String())
}

// Broadcast sends data to all connected clients
func (s *Server) Broadcast(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, conn := range s.connections {
		conn.Send(data)
	}
}
