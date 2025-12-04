package config

import (
	"encoding/hex"
	"fmt"
	"os"

	"seras-protocol/internal/transport/client/udp"
	"seras-protocol/internal/transport/client/wss"
	"seras-protocol/pkg/taiga/msg"
)

// TransportConfig is interface for transport-specific configuration
type TransportConfig interface {
	GetFromEnv() error
}

var ConnTypeMap = map[string]func() TransportConfig{
	"wss": func() TransportConfig { return &wss.Config{} },
	"udp": func() TransportConfig { return &udp.Config{} },
}

type ConnConfig struct {
	PrivateKey      msg.Key         // Client's private key
	NodePublicKey   msg.Key         // Node's public key (for encryption)
	Type            string          // Transport type (e.g., "wss")
	LocalIP         string          // IP for TUN interface (e.g., "11.0.0.2")
	NodeVPNIP       string          // Node's VPN IP (e.g., "11.0.0.1")
	GatewayIP       string          // Gateway to route node traffic
	RemoteHost      string          // Node public IP (to exclude from TUN routing)
	TransportConfig TransportConfig // Transport-specific config
}

func ParseConfigFromEnv(connType string) (*ConnConfig, error) {
	configFactory, ok := ConnTypeMap[connType]
	if !ok {
		return nil, fmt.Errorf("invalid connection type: %s", connType)
	}
	transportConfig := configFactory()
	if err := transportConfig.GetFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to get transport config: %w", err)
	}

	// Parse private key
	privKeyHex := os.Getenv("PRIVATE_KEY")
	if privKeyHex == "" {
		return nil, fmt.Errorf("PRIVATE_KEY is not set")
	}
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil || len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("PRIVATE_KEY must be 32 bytes hex")
	}
	var privateKey msg.Key
	copy(privateKey[:], privKeyBytes)

	// Parse node public key
	nodePubKeyHex := os.Getenv("NODE_PUBLIC_KEY")
	if nodePubKeyHex == "" {
		return nil, fmt.Errorf("NODE_PUBLIC_KEY is not set")
	}
	nodePubKeyBytes, err := hex.DecodeString(nodePubKeyHex)
	if err != nil || len(nodePubKeyBytes) != 32 {
		return nil, fmt.Errorf("NODE_PUBLIC_KEY must be 32 bytes hex")
	}
	var nodePublicKey msg.Key
	copy(nodePublicKey[:], nodePubKeyBytes)

	// Network config
	localIP := os.Getenv("LOCAL_IP")
	if localIP == "" {
		return nil, fmt.Errorf("LOCAL_IP is not set")
	}

	nodeVPNIP := os.Getenv("NODE_VPN_IP")
	if nodeVPNIP == "" {
		return nil, fmt.Errorf("NODE_VPN_IP is not set (node's TUN IP, e.g., 11.0.0.1)")
	}

	gatewayIP := os.Getenv("GATEWAY_IP")
	if gatewayIP == "" {
		return nil, fmt.Errorf("GATEWAY_IP is not set")
	}

	remoteHost := os.Getenv("REMOTE_HOST")
	if remoteHost == "" {
		return nil, fmt.Errorf("REMOTE_HOST is not set")
	}

	return &ConnConfig{
		PrivateKey:      privateKey,
		NodePublicKey:   nodePublicKey,
		Type:            connType,
		LocalIP:         localIP,
		NodeVPNIP:       nodeVPNIP,
		GatewayIP:       gatewayIP,
		RemoteHost:      remoteHost,
		TransportConfig: transportConfig,
	}, nil
}

func GetConnTypeFromEnv() (string, error) {
	env := os.Getenv("CONN_TYPE")
	if env == "" {
		return "", fmt.Errorf("CONN_TYPE is not set")
	}
	if _, ok := ConnTypeMap[env]; !ok {
		return "", fmt.Errorf("invalid connection type: %s", env)
	}
	return env, nil
}
