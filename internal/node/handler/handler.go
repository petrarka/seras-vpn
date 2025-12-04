package handler

import (
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
	tun        *tun.TUN
	decoder    *msg.Decoder
	privateKey msg.Key
	// Map connection to its encoder (for responses)
	connEncoders map[*wss.Connection]*msg.Encoder
	mu           sync.RWMutex
}

// NewHandler creates a new packet handler
func NewHandler(t *tun.TUN, privateKey msg.Key) *Handler {
	return &Handler{
		tun:          t,
		decoder:      msg.NewDecoder(privateKey),
		privateKey:   privateKey,
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

	// Check message type
	switch rawMsg.Header.Type {
	case msg.TypeHandshake:
		h.handleHandshake(conn, rawMsg)
	case msg.TypeData:
		h.handleData(conn, rawMsg)
	default:
		slog.Warn("Unknown message type", "type", rawMsg.Header.Type)
	}
}

// handleHandshake processes client handshake and stores their public key
func (h *Handler) handleHandshake(conn *wss.Connection, rawMsg *msg.RawMsg) {
	// Decrypt handshake
	hs, err := h.decoder.DecryptHandshake(rawMsg)
	if err != nil {
		slog.Error("Failed to decrypt handshake", "error", err)
		h.sendHandshakeAck(conn, nil, false, "decrypt error")
		return
	}

	// Store encoder for this client's public key
	h.mu.Lock()
	h.connEncoders[conn] = msg.NewEncoder(hs.ClientPublicKey)
	h.mu.Unlock()

	slog.Info("Client registered", "pubkey", hs.ClientPublicKey[:8])

	// Send ack
	h.sendHandshakeAck(conn, &hs.ClientPublicKey, true, "ok")
}

// sendHandshakeAck sends handshake acknowledgment to client
func (h *Handler) sendHandshakeAck(conn *wss.Connection, clientPubKey *msg.Key, success bool, message string) {
	ack := &msg.HandshakeAck{
		Success: success,
		Message: message,
	}

	// If we don't have client's public key, we can't send encrypted ack
	if clientPubKey == nil {
		slog.Error("Cannot send ack - no client public key")
		return
	}

	encoder := msg.NewEncoder(*clientPubKey)
	rawMsg, err := encoder.EncryptHandshakeAck(ack)
	if err != nil {
		slog.Error("Failed to encrypt ack", "error", err)
		return
	}

	data, err := binary.Marshal(rawMsg)
	if err != nil {
		slog.Error("Failed to marshal ack", "error", err)
		return
	}

	conn.Send(data)
}

// handleData processes VPN data packet
func (h *Handler) handleData(conn *wss.Connection, rawMsg *msg.RawMsg) {
	// Check if client has completed handshake
	h.mu.RLock()
	_, hasEncoder := h.connEncoders[conn]
	h.mu.RUnlock()

	if !hasEncoder {
		slog.Warn("Data from unregistered client, ignoring")
		return
	}

	// Decrypt message
	cookedMsg, err := h.decoder.DecryptBody(rawMsg)
	if err != nil {
		slog.Error("Failed to decrypt message", "error", err)
		return
	}

	// Check if this is final destination or needs forwarding
	if cookedMsg.Body.NextHop != nil {
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

// StartTUNReader reads from TUN and sends to connected clients
func (h *Handler) StartTUNReader(server *wss.Server) {
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

		// Send to all registered clients with their specific encoders
		h.mu.RLock()
		for conn, encoder := range h.connEncoders {
			rawMsg, err := encoder.EncryptMsg(message)
			if err != nil {
				slog.Error("Failed to encrypt response", "error", err)
				continue
			}

			data, err := binary.Marshal(rawMsg)
			if err != nil {
				slog.Error("Failed to marshal response", "error", err)
				continue
			}

			conn.Send(data)
		}
		h.mu.RUnlock()
	}
}

// RemoveConnection removes encoder for disconnected client
func (h *Handler) RemoveConnection(conn *wss.Connection) {
	h.mu.Lock()
	delete(h.connEncoders, conn)
	h.mu.Unlock()
	slog.Info("Client disconnected")
}
