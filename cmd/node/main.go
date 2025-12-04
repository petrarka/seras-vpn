package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"seras-protocol/internal/node/config"
	"seras-protocol/internal/node/handler"
	"seras-protocol/internal/transport/server/udp"
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
	slog.Info("Config loaded",
		"transport", cfg.TransportType,
		"listenAddr", cfg.ListenAddr,
		"tunIP", cfg.TunIP,
		"vpnSubnet", cfg.VPNSubnet)

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

	// Start TUN reader in background
	go h.StartTUNReader()

	// Start server based on transport type
	switch cfg.TransportType {
	case "wss":
		startWSSServer(cfg, h)
	case "udp":
		startUDPServer(cfg, h)
	default:
		slog.Error("Unknown transport type", "type", cfg.TransportType)
		os.Exit(1)
	}
}

func startWSSServer(cfg *config.NodeConfig, h *handler.Handler) {
	server := wss.NewServer(cfg.ListenAddr, func(conn *wss.Connection, data []byte) {
		h.HandleMessage(conn, data)
	})
	server.SetOnDisconnect(func(conn *wss.Connection) {
		h.RemoveConnection(conn)
	})

	slog.Info("Starting WSS server", "addr", cfg.ListenAddr)
	if err := server.Start(); err != nil {
		slog.Error("WSS server error", "error", err)
		os.Exit(1)
	}
}

func startUDPServer(cfg *config.NodeConfig, h *handler.Handler) {
	server := udp.NewServer(cfg.ListenAddr, func(conn *udp.Connection, data []byte) {
		h.HandleMessage(conn, data)
	})
	server.SetOnDisconnect(func(conn *udp.Connection) {
		h.RemoveConnection(conn)
	})

	slog.Info("Starting UDP server", "addr", cfg.ListenAddr)
	if err := server.Start(); err != nil {
		slog.Error("UDP server error", "error", err)
		os.Exit(1)
	}
}
