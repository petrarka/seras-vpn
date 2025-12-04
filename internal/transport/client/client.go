package client

import (
	"fmt"

	"seras-protocol/internal/transport/client/udp"
	"seras-protocol/internal/transport/client/wss"
)

type Client interface {
	Disconnect() error
	Send(data []byte) error
	Receive() ([]byte, error)
}

type Config interface {
	GetFromEnv() error
}

type Factory struct{}

func (f *Factory) NewClient(connType string, transportConfig Config) (Client, error) {
	switch connType {
	case "wss":
		wssConfig, ok := transportConfig.(*wss.Config)
		if !ok {
			return nil, fmt.Errorf("invalid wss config type")
		}
		return wss.NewTransport(wssConfig)
	case "udp":
		udpConfig, ok := transportConfig.(*udp.Config)
		if !ok {
			return nil, fmt.Errorf("invalid udp config type")
		}
		return udp.NewTransport(udpConfig)
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", connType)
	}
}
