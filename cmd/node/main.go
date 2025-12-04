package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"seras-protocol/internal/node/config"
	"seras-protocol/internal/node/handler"
	"seras-protocol/internal/transport/server/wss"
	"seras-protocol/internal/tun"
)

func main() {
	slog.Info("Starting Seras Node")

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found", "error", err)
	}

	cfg, err := config.ParseNodeConfigFromEnv()
	if err != nil {
		slog.Error("Failed to parse config", "error", err)
		os.Exit(1)
	}
	slog.Info("Config loaded", "listenAddr", cfg.ListenAddr, "tunIP", cfg.TunIP, "vpnSubnet", cfg.VPNSubnet)

	// Create TUN interface for node with routing and NAT
	tunDev, err := tun.NewNodeTUN(cfg.TunIP, cfg.VPNSubnet)
	if err != nil {
		slog.Error("Failed to create TUN interface", "error", err)
		os.Exit(1)
	}
	defer tunDev.Close()
	slog.Info("TUN interface created", "name", tunDev.Name())

	// Create handler
	h := handler.NewHandler(tunDev, cfg.PrivateKey)

	// Create WebSocket server
	server := wss.NewServer(cfg.ListenAddr, h.HandleMessage)
	server.SetOnDisconnect(h.RemoveConnection)

	// Start TUN reader in background (client keys are received via handshake)
	go h.StartTUNReader(server)

	// Start WebSocket server (blocking)
	slog.Info("Node running, waiting for client handshakes...")
	if err := server.Start(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
