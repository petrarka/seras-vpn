package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"seras-protocol/pkg/taiga/msg"
)

type NodeConfig struct {
	PrivateKey msg.Key // Node's private key for decryption
	PublicKey  msg.Key // Node's public key (derived or provided)
	ListenAddr string  // WebSocket listen address (e.g., ":8080")
	TunIP      string  // IP for node's TUN interface (e.g., "11.0.0.1")
	VPNSubnet  string  // VPN subnet for clients (e.g., "11.0.0.0/24")
}

func ParseNodeConfigFromEnv() (*NodeConfig, error) {
	// Parse private key
	privKeyHex := os.Getenv("NODE_PRIVATE_KEY")
	if privKeyHex == "" {
		return nil, fmt.Errorf("NODE_PRIVATE_KEY is not set")
	}
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil || len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("NODE_PRIVATE_KEY must be 32 bytes hex")
	}
	var privateKey msg.Key
	copy(privateKey[:], privKeyBytes)

	// Parse public key (optional)
	var publicKey msg.Key
	pubKeyHex := os.Getenv("NODE_PUBLIC_KEY")
	if pubKeyHex != "" {
		pubKeyBytes, err := hex.DecodeString(pubKeyHex)
		if err != nil || len(pubKeyBytes) != 32 {
			return nil, fmt.Errorf("NODE_PUBLIC_KEY must be 32 bytes hex")
		}
		copy(publicKey[:], pubKeyBytes)
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	tunIP := os.Getenv("TUN_IP")
	if tunIP == "" {
		return nil, fmt.Errorf("TUN_IP is not set")
	}

	vpnSubnet := os.Getenv("VPN_SUBNET")
	if vpnSubnet == "" {
		return nil, fmt.Errorf("VPN_SUBNET is not set (e.g., 11.0.0.0/24)")
	}

	return &NodeConfig{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		ListenAddr: listenAddr,
		TunIP:      tunIP,
		VPNSubnet:  vpnSubnet,
	}, nil
}
