package udp

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"
)

type Config struct {
	Addr string
}

func (c *Config) GetFromEnv() error {
	c.Addr = os.Getenv("UDP_ADDR")
	if c.Addr == "" {
		return fmt.Errorf("UDP_ADDR is not set")
	}
	slog.Info("UDP address configured", "addr", c.Addr)
	return nil
}

type Transport struct {
	conn       *net.UDPConn
	serverAddr *net.UDPAddr
}

func NewTransport(config *Config) (*Transport, error) {
	slog.Info("Connecting to UDP server", "addr", config.Addr)

	serverAddr, err := net.ResolveUDPAddr("udp", config.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial UDP: %w", err)
	}

	slog.Info("UDP connected", "local", conn.LocalAddr(), "remote", serverAddr)
	return &Transport{conn: conn, serverAddr: serverAddr}, nil
}

func (t *Transport) Disconnect() error {
	slog.Info("Disconnecting UDP")
	return t.conn.Close()
}

func (t *Transport) Send(data []byte) error {
	_, err := t.conn.Write(data)
	return err
}

func (t *Transport) Receive() ([]byte, error) {
	buf := make([]byte, 65535)
	t.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	n, err := t.conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read UDP: %w", err)
	}
	return buf[:n], nil
}
