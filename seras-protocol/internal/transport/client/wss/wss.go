package wss

import (
	"fmt"
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
	return nil
}

type Transport struct {
	conn *websocket.Conn
}

func NewTransport(config *Config) (*Transport, error) {

	conn, _, err := websocket.DefaultDialer.Dial(config.Url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return &Transport{conn}, nil
}

func (t *Transport) Disconnect() error {
	return t.conn.Close()
}

func (t *Transport) Send(data []byte) error {
	return t.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (t *Transport) Receive() ([]byte, error) {
	tp, data, err := t.conn.ReadMessage()
	if tp != websocket.BinaryMessage {
		return nil, fmt.Errorf("received message is not binary")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %v", err)
	}
	return data, nil
}
