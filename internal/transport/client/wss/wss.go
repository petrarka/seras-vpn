package wss

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gorilla/websocket"
	"os"
)

type Config struct {
	Url string
}

func (c *Config) GetFromEnv() error {
	c.Url = os.Getenv("WS_URL")
	if c.Url == "" {
		return fmt.Errorf("WS_URL is not set")
	}

	// Validate URL format
	if !strings.HasPrefix(c.Url, "ws://") && !strings.HasPrefix(c.Url, "wss://") {
		return fmt.Errorf("WS_URL must start with ws:// or wss://, got: %s", c.Url)
	}

	// Auto-add /ws if missing
	if !strings.HasSuffix(c.Url, "/ws") {
		c.Url = strings.TrimSuffix(c.Url, "/") + "/ws"
	}

	slog.Info("WebSocket URL configured", "url", c.Url)
	return nil
}

type Transport struct {
	conn *websocket.Conn
}

func NewTransport(config *Config) (*Transport, error) {
	slog.Info("Connecting to WebSocket", "url", config.Url)

	conn, resp, err := websocket.DefaultDialer.Dial(config.Url, nil)
	if err != nil {
		if resp != nil {
			slog.Error("WebSocket dial failed", "status", resp.Status, "statusCode", resp.StatusCode)
			return nil, fmt.Errorf("failed to connect: %v (HTTP %d)", err, resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	slog.Info("WebSocket connected")
	return &Transport{conn}, nil
}

func (t *Transport) Disconnect() error {
	slog.Info("Disconnecting WebSocket")
	return t.conn.Close()
}

func (t *Transport) Send(data []byte) error {
	return t.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (t *Transport) Receive() ([]byte, error) {
	tp, data, err := t.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %v", err)
	}
	if tp != websocket.BinaryMessage {
		return nil, fmt.Errorf("received non-binary message type: %d", tp)
	}
	return data, nil
}
