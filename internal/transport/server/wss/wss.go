package wss

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1500,
	WriteBufferSize: 1500,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for VPN
	},
}

// Connection represents a single WebSocket client connection
type Connection struct {
	conn   *websocket.Conn
	sendCh chan []byte
	mu     sync.Mutex
	closed bool
}

// Server is a WebSocket server for node
type Server struct {
	addr         string
	connections  map[*Connection]bool
	mu           sync.RWMutex
	onMessage    func(conn *Connection, data []byte)
	onDisconnect func(conn *Connection)
}

// NewServer creates a new WebSocket server
func NewServer(addr string, onMessage func(conn *Connection, data []byte)) *Server {
	return &Server{
		addr:        addr,
		connections: make(map[*Connection]bool),
		onMessage:   onMessage,
	}
}

// SetOnDisconnect sets callback for client disconnection
func (s *Server) SetOnDisconnect(callback func(conn *Connection)) {
	s.onDisconnect = callback
}

// Start starts the WebSocket server
func (s *Server) Start() error {
	http.HandleFunc("/ws", s.handleWebSocket)
	slog.Info("WebSocket server starting", "addr", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade connection", "error", err)
		return
	}

	conn := &Connection{
		conn:   ws,
		sendCh: make(chan []byte, 256),
	}

	s.mu.Lock()
	s.connections[conn] = true
	s.mu.Unlock()

	slog.Info("Client connected", "remote", r.RemoteAddr)

	// Start writer goroutine
	go conn.writePump()

	// Read messages in current goroutine
	conn.readPump(s)

	// Cleanup - mark closed before closing channel
	conn.mu.Lock()
	conn.closed = true
	conn.mu.Unlock()

	// Notify handler before removing connection
	if s.onDisconnect != nil {
		s.onDisconnect(conn)
	}

	s.mu.Lock()
	delete(s.connections, conn)
	s.mu.Unlock()

	close(conn.sendCh)
	ws.Close()
	slog.Info("Client disconnected", "remote", r.RemoteAddr)
}

func (c *Connection) readPump(s *Server) {
	for {
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("Read error", "error", err)
			}
			return
		}

		if msgType != websocket.BinaryMessage {
			continue
		}

		if s.onMessage != nil {
			s.onMessage(c, data)
		}
	}
}

func (c *Connection) writePump() {
	for data := range c.sendCh {
		c.mu.Lock()
		err := c.conn.WriteMessage(websocket.BinaryMessage, data)
		c.mu.Unlock()
		if err != nil {
			slog.Error("Write error", "error", err)
			return
		}
	}
}

// Send sends data to the client
func (c *Connection) Send(data []byte) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("connection closed")
	}
	c.mu.Unlock()

	select {
	case c.sendCh <- data:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

// Broadcast sends data to all connected clients
func (s *Server) Broadcast(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for conn := range s.connections {
		conn.Send(data)
	}
}
