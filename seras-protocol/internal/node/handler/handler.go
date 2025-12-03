package handler

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kelindar/binary"
	"seras-protocol/internal/transport/server/wss"
	"seras-protocol/internal/tun"
	"seras-protocol/pkg/taiga/msg"
)

// Handler processes packets between clients and TUN interface
type Handler struct {
	tun     *tun.TUN
	decoder *msg.Decoder
	// Map connection to its encoder (for responses)
	// In future: map client public key to encoder for multi-hop
	connEncoders map[*wss.Connection]*msg.Encoder
	mu           sync.RWMutex
}

// NewHandler creates a new packet handler
func NewHandler(t *tun.TUN, privateKey msg.Key) *Handler {
	return &Handler{
		tun:          t,
		decoder:      msg.NewDecoder(privateKey),
		connEncoders: make(map[*wss.Connection]*msg.Encoder),
	}
}

// HandleMessage processes incoming encrypted message from client
func (h *Handler) HandleMessage(conn *wss.Connection, data []byte) {
	// Unmarshal wire format
	rawMsg := &msg.RawMsg{}
	if err := binary.Unmarshal(data, rawMsg); err != nil {
		slog.Error("Failed to unmarshal message", "error", err)
		return
	}

	// Store encoder for this connection using client's ephemeral key
	// This allows us to respond back to the client
	h.mu.Lock()
	if _, exists := h.connEncoders[conn]; !exists {
		// For responses, we use the client's ephemeral public key
		h.connEncoders[conn] = msg.NewEncoder(rawMsg.Header.EphemeralKey)
	}
	h.mu.Unlock()

	// Decrypt message
	cookedMsg, err := h.decoder.DecryptBody(rawMsg)
	if err != nil {
		slog.Error("Failed to decrypt message", "error", err)
		return
	}

	// Check if this is final destination or needs forwarding
	if cookedMsg.Body.NextHop != nil {
		// TODO: Multi-hop routing - forward to next node
		slog.Warn("Multi-hop routing not implemented yet")
		return
	}

	// Final destination - write IP packet to TUN
	n, err := h.tun.Write(cookedMsg.Body.Data)
	if err != nil {
		slog.Error("Failed to write to TUN", "error", err)
		return
	}
	if n != len(cookedMsg.Body.Data) {
		slog.Warn("Incomplete TUN write", "written", n, "expected", len(cookedMsg.Body.Data))
	}
}

// StartTUNReader reads from TUN and sends to all connected clients
// In production, you'd route packets to specific clients based on IP
func (h *Handler) StartTUNReader(server *wss.Server, clientPublicKey msg.Key) {
	encoder := msg.NewEncoder(clientPublicKey)
	buf := make([]byte, 1500)

	for {
		n, err := h.tun.Read(buf)
		if err != nil {
			slog.Error("Failed to read from TUN", "error", err)
			continue
		}

		if n == 0 {
			continue
		}

		// Create response message
		message := &msg.Msg{
			Flags:     0,
			Timestamp: time.Now().Unix(),
			NextHop:   nil,
			Data:      buf[:n],
		}

		// Encrypt for client
		rawMsg, err := encoder.EncryptMsg(message)
		if err != nil {
			slog.Error("Failed to encrypt response", "error", err)
			continue
		}

		// Marshal to wire format
		data, err := binary.Marshal(rawMsg)
		if err != nil {
			slog.Error("Failed to marshal response", "error", err)
			continue
		}

		// Send to all connected clients
		// TODO: Route to specific client based on destination IP
		server.Broadcast(data)
	}
}

// RemoveConnection removes encoder for disconnected client
func (h *Handler) RemoveConnection(conn *wss.Connection) {
	h.mu.Lock()
	delete(h.connEncoders, conn)
	h.mu.Unlock()
}

// GetEncoder returns encoder for a connection
func (h *Handler) GetEncoder(conn *wss.Connection) (*msg.Encoder, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	enc, exists := h.connEncoders[conn]
	if !exists {
		return nil, fmt.Errorf("no encoder for connection")
	}
	return enc, nil
}
