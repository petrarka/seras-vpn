package vpn

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kelindar/binary"
	"seras-protocol/internal/kedr/config"
	"seras-protocol/internal/kedr/processor"
	"seras-protocol/internal/transport/client"
	"seras-protocol/internal/tun"
	"seras-protocol/pkg/taiga/msg"
)

// Node represents a hop in the circuit
type Node struct {
	PublicKey msg.Key
	Protocol  msg.Protocol
	Endpoint  string
}

// Circuit is a chain of nodes (currently supports 1, designed for multiple)
type Circuit struct {
	Nodes []*Node
}

// Client is the VPN client that handles TUN <-> WebSocket communication
type Client struct {
	tun           *tun.TUN
	transport     client.Client
	encoder       *msg.Encoder
	decoder       *msg.Decoder
	processor     *processor.Processor
	circuit       *Circuit
	clientPubKey  msg.Key
}

// NewClient creates a new VPN client
func NewClient(cfg *config.ConnConfig, t *tun.TUN, transport client.Client) *Client {
	// Create circuit with single node (for now)
	circuit := &Circuit{
		Nodes: []*Node{{
			PublicKey: cfg.NodePublicKey,
			Protocol:  msg.Protocol(cfg.Type),
			Endpoint:  cfg.RemoteHost,
		}},
	}

	// Derive client public key from private key
	clientPubKey, _ := msg.PublicKeyFromPrivate(cfg.PrivateKey)

	return &Client{
		tun:          t,
		transport:    transport,
		encoder:      msg.NewEncoder(cfg.NodePublicKey),
		decoder:      msg.NewDecoder(cfg.PrivateKey),
		processor:    processor.NewProcessor(t),
		circuit:      circuit,
		clientPubKey: clientPubKey,
	}
}

// Run starts both send and receive loops
func (c *Client) Run(ctx context.Context) error {
	// Perform handshake first
	if err := c.handshake(); err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}
	slog.Info("Handshake complete")

	errChan := make(chan error, 2)

	go c.sendLoop(ctx, errChan)
	go c.receiveLoop(ctx, errChan)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

// handshake sends client public key to node and waits for ack
func (c *Client) handshake() error {
	// Create handshake message with our public key
	hs := &msg.Handshake{
		ClientPublicKey: c.clientPubKey,
	}

	// Encrypt handshake for node
	rawMsg, err := c.encoder.EncryptHandshake(hs)
	if err != nil {
		return fmt.Errorf("encrypt handshake: %w", err)
	}

	// Marshal and send
	data, err := binary.Marshal(rawMsg)
	if err != nil {
		return fmt.Errorf("marshal handshake: %w", err)
	}

	if err := c.transport.Send(data); err != nil {
		return fmt.Errorf("send handshake: %w", err)
	}

	// Wait for ack
	ackData, err := c.transport.Receive()
	if err != nil {
		return fmt.Errorf("receive ack: %w", err)
	}

	// Unmarshal ack
	ackRaw := &msg.RawMsg{}
	if err := binary.Unmarshal(ackData, ackRaw); err != nil {
		return fmt.Errorf("unmarshal ack: %w", err)
	}

	// Check message type
	if ackRaw.Header.Type != msg.TypeHandshakeAck {
		return fmt.Errorf("expected handshake ack, got type %d", ackRaw.Header.Type)
	}

	// Decrypt ack
	ack, err := c.decoder.DecryptHandshakeAck(ackRaw)
	if err != nil {
		return fmt.Errorf("decrypt ack: %w", err)
	}

	if !ack.Success {
		return fmt.Errorf("handshake rejected: %s", ack.Message)
	}

	return nil
}

// sendLoop reads from TUN, encrypts and sends via WebSocket
func (c *Client) sendLoop(ctx context.Context, errChan chan<- error) {
	buf := make([]byte, 1500) // MTU size buffer

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := c.tun.Read(buf)
		if err != nil {
			slog.Error("failed to read from TUN", "error", err)
			errChan <- fmt.Errorf("TUN read error: %w", err)
			return
		}

		if n == 0 {
			continue
		}

		// Create message with IP packet data
		message := &msg.Msg{
			Flags:     0,
			Timestamp: time.Now().Unix(),
			NextHop:   nil, // Direct to node (single hop for now)
			Data:      buf[:n],
		}

		// Encrypt message
		rawMsg, err := c.encoder.EncryptMsg(message)
		if err != nil {
			slog.Error("failed to encrypt message", "error", err)
			continue
		}

		// Marshal to wire format
		data, err := binary.Marshal(rawMsg)
		if err != nil {
			slog.Error("failed to marshal message", "error", err)
			continue
		}

		// Send via transport
		if err := c.transport.Send(data); err != nil {
			slog.Error("failed to send message", "error", err)
			errChan <- fmt.Errorf("transport send error: %w", err)
			return
		}
	}
}

// receiveLoop receives from WebSocket, decrypts and writes to TUN
func (c *Client) receiveLoop(ctx context.Context, errChan chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		data, err := c.transport.Receive()
		if err != nil {
			slog.Error("failed to receive message", "error", err)
			errChan <- fmt.Errorf("transport receive error: %w", err)
			return
		}

		// Unmarshal wire format
		rawMsg := &msg.RawMsg{}
		if err := binary.Unmarshal(data, rawMsg); err != nil {
			slog.Error("failed to unmarshal message", "error", err)
			continue
		}

		// Decrypt message
		cookedMsg, err := c.decoder.DecryptBody(rawMsg)
		if err != nil {
			slog.Error("failed to decrypt message", "error", err)
			continue
		}

		// Process (write to TUN)
		if err := c.processor.Process(cookedMsg); err != nil {
			slog.Error("failed to process message", "error", err)
			continue
		}
	}
}

// Close closes all resources
func (c *Client) Close() error {
	if err := c.transport.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect transport: %w", err)
	}
	return c.tun.Close()
}
