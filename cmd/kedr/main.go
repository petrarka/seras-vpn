package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"seras-protocol/internal/kedr/config"
	"seras-protocol/internal/kedr/vpn"
	"seras-protocol/internal/transport/client"
	"seras-protocol/internal/tun"
)

func main() {
	slog.Info("Starting Kedr VPN client")

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found", "error", err)
	}

	connType, err := config.GetConnTypeFromEnv()
	if err != nil {
		slog.Error("Failed to get connection type", "error", err)
		os.Exit(1)
	}
	slog.Info("Connection type", "type", connType)

	cfg, err := config.ParseConfigFromEnv(connType)
	if err != nil {
		slog.Error("Failed to parse config", "error", err)
		os.Exit(1)
	}
	slog.Info("Config loaded", "localIP", cfg.LocalIP, "nodeVPNIP", cfg.NodeVPNIP, "remoteHost", cfg.RemoteHost)

	// Create TUN interface
	tunDev, err := tun.New(cfg.LocalIP, cfg.GatewayIP, cfg.RemoteHost, cfg.NodeVPNIP)
	if err != nil {
		slog.Error("Failed to create TUN interface", "error", err)
		os.Exit(1)
	}
	slog.Info("TUN interface created", "name", tunDev.Name())

	// Create transport
	factory := &client.Factory{}
	transport, err := factory.NewClient(cfg.Type, cfg.TransportConfig)
	if err != nil {
		tunDev.Close()
		slog.Error("Failed to create transport", "error", err)
		os.Exit(1)
	}
	slog.Info("Transport connected")

	// Create VPN client
	vpnClient := vpn.NewClient(cfg, tunDev, transport)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Run VPN client
	slog.Info("VPN client running")
	if err := vpnClient.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("VPN client error", "error", err)
	}

	// Cleanup
	if err := vpnClient.Close(); err != nil {
		slog.Error("Failed to close VPN client", "error", err)
	}

	slog.Info("Kedr VPN client stopped")
}
