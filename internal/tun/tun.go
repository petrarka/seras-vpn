package tun

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/songgao/water"
)

type TUN struct {
	dev            *water.Interface
	name           string
	localIP        string
	peerIP         string
	subnet         string // e.g., "11.0.0.0/24"
	isNode         bool
	nodeIP         string // for client cleanup
	gateway        string // for client cleanup
	dnsServers     []string // DNS servers to use
	originalDNS    []string // Original DNS to restore
	networkService string   // macOS network service name
}

// New creates TUN for client and routes all traffic through it
func New(localIP, gateway, nodeIP, nodeVPNIP string) (*TUN, error) {
	return NewWithDNS(localIP, gateway, nodeIP, nodeVPNIP, []string{"8.8.8.8", "1.1.1.1"})
}

// NewWithDNS creates TUN for client with custom DNS servers
func NewWithDNS(localIP, gateway, nodeIP, nodeVPNIP string, dnsServers []string) (*TUN, error) {
	dev, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}

	t := &TUN{
		dev:        dev,
		name:       dev.Name(),
		localIP:    localIP,
		peerIP:     nodeVPNIP, // Node's TUN IP
		isNode:     false,
		nodeIP:     nodeIP,
		gateway:    gateway,
		dnsServers: dnsServers,
	}

	if err := t.setupClient(gateway, nodeIP); err != nil {
		dev.Close()
		return nil, fmt.Errorf("setup tun: %w", err)
	}

	return t, nil
}

// NewNodeTUN creates TUN for node (exit node) with NAT and routing
func NewNodeTUN(localIP, vpnSubnet string) (*TUN, error) {
	dev, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}

	t := &TUN{
		dev:     dev,
		name:    dev.Name(),
		localIP: localIP,
		subnet:  vpnSubnet,
		isNode:  true,
	}

	if err := t.setupNode(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("setup node tun: %w", err)
	}

	return t, nil
}

func (t *TUN) setupClient(gateway, nodeIP string) error {
	if runtime.GOOS == "darwin" {
		return t.setupClientDarwin(gateway, nodeIP)
	}
	return t.setupClientLinux(gateway, nodeIP)
}

func (t *TUN) setupClientLinux(gateway, nodeIP string) error {
	cmds := [][]string{
		{"ip", "addr", "add", t.localIP + "/24", "dev", t.name},
		{"ip", "link", "set", t.name, "mtu", "1300"},
		{"ip", "link", "set", t.name, "up"},
		{"ip", "route", "add", nodeIP + "/32", "via", gateway},
		{"ip", "route", "add", "0.0.0.0/1", "dev", t.name},
		{"ip", "route", "add", "128.0.0.0/1", "dev", t.name},
	}

	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			// Ignore "File exists" for routes (from previous run)
			if !strings.Contains(string(out), "File exists") {
				return fmt.Errorf("%v: %w (%s)", args, err, string(out))
			}
		}
	}
	return nil
}

func (t *TUN) setupClientDarwin(gateway, nodeIP string) error {
	cmds := [][]string{
		{"ifconfig", t.name, "inet", t.localIP, t.peerIP, "up"},
		{"ifconfig", t.name, "mtu", "1300"},
		{"route", "add", "-host", nodeIP, gateway},
		{"route", "add", "-net", "0.0.0.0/1", t.peerIP},
		{"route", "add", "-net", "128.0.0.0/1", t.peerIP},
	}

	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %w (%s)", args, err, string(out))
		}
	}

	// Setup DNS if servers specified
	if len(t.dnsServers) > 0 {
		if err := t.setupDNSDarwin(); err != nil {
			fmt.Printf("Warning: DNS setup failed: %v\n", err)
		}
	}

	return nil
}

func (t *TUN) setupDNSDarwin() error {
	// Find active network service
	t.networkService = getActiveNetworkService()
	if t.networkService == "" {
		return fmt.Errorf("could not detect active network service")
	}

	// Save original DNS
	out, err := exec.Command("networksetup", "-getdnsservers", t.networkService).Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.Contains(line, "aren't any") {
				t.originalDNS = append(t.originalDNS, line)
			}
		}
	}

	// Set new DNS
	args := append([]string{"-setdnsservers", t.networkService}, t.dnsServers...)
	if out, err := exec.Command("networksetup", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("set dns: %w (%s)", err, string(out))
	}

	fmt.Printf("DNS set to %v (was: %v) on %s\n", t.dnsServers, t.originalDNS, t.networkService)
	return nil
}

func (t *TUN) restoreDNSDarwin() {
	if t.networkService == "" {
		return
	}

	var args []string
	if len(t.originalDNS) == 0 {
		// Restore to DHCP
		args = []string{"-setdnsservers", t.networkService, "empty"}
	} else {
		args = append([]string{"-setdnsservers", t.networkService}, t.originalDNS...)
	}

	exec.Command("networksetup", args...).Run()
	fmt.Printf("DNS restored on %s\n", t.networkService)
}

func getActiveNetworkService() string {
	// Try common services in order
	services := []string{"Wi-Fi", "Ethernet", "USB 10/100/1000 LAN", "Thunderbolt Ethernet"}

	for _, svc := range services {
		out, err := exec.Command("networksetup", "-getinfo", svc).Output()
		if err == nil && strings.Contains(string(out), "IP address:") {
			return svc
		}
	}

	// Fallback: try to get from route
	out, _ := exec.Command("route", "-n", "get", "default").Output()
	if strings.Contains(string(out), "interface:") {
		// Try to match interface to service
		for _, svc := range services {
			out2, _ := exec.Command("networksetup", "-getinfo", svc).Output()
			if len(out2) > 0 {
				return svc
			}
		}
	}

	return "Wi-Fi" // Default fallback
}

func (t *TUN) setupNode() error {
	if runtime.GOOS == "darwin" {
		return t.setupNodeDarwin()
	}
	return t.setupNodeLinux()
}

func (t *TUN) setupNodeLinux() error {
	// Get subnet base (e.g., "11.0.0.0/24" -> "11.0.0")
	subnetBase := getSubnetBase(t.subnet)

	cmds := [][]string{
		{"ip", "addr", "add", t.localIP + "/24", "dev", t.name},
		{"ip", "link", "set", t.name, "mtu", "1300"},
		{"ip", "link", "set", t.name, "up"},
		// Route for VPN subnet through TUN
		{"ip", "route", "add", t.subnet, "dev", t.name},
	}

	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			// Ignore "File exists" errors for routes
			if !strings.Contains(string(out), "File exists") {
				return fmt.Errorf("%v: %w (%s)", args, err, string(out))
			}
		}
	}

	// Enable IP forwarding
	if out, err := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").CombinedOutput(); err != nil {
		return fmt.Errorf("enable ip forwarding: %w (%s)", err, string(out))
	}

	// Setup NAT for VPN subnet (check if rule exists first)
	if err := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", t.subnet, "-j", "MASQUERADE").Run(); err != nil {
		// Rule doesn't exist, add it
		if out, err := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", t.subnet, "-j", "MASQUERADE").CombinedOutput(); err != nil {
			return fmt.Errorf("setup nat: %w (%s)", err, string(out))
		}
	}

	fmt.Printf("Node TUN setup complete: %s, subnet: %s, base: %s\n", t.name, t.subnet, subnetBase)
	return nil
}

func (t *TUN) setupNodeDarwin() error {
	// Extract first client IP from subnet for point-to-point
	peerIP := getFirstClientIP(t.subnet)

	cmds := [][]string{
		{"ifconfig", t.name, "inet", t.localIP, peerIP, "up"},
		{"ifconfig", t.name, "mtu", "1300"},
		{"route", "add", "-net", t.subnet, "-interface", t.name},
	}

	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			if !strings.Contains(string(out), "File exists") {
				return fmt.Errorf("%v: %w (%s)", args, err, string(out))
			}
		}
	}

	// Enable IP forwarding
	if out, err := exec.Command("sysctl", "-w", "net.inet.ip.forwarding=1").CombinedOutput(); err != nil {
		return fmt.Errorf("enable ip forwarding: %w (%s)", err, string(out))
	}

	// Setup NAT with pfctl
	natRule := fmt.Sprintf("nat on en0 from %s to any -> (en0)\n", t.subnet)
	if err := setupPfNat(natRule); err != nil {
		fmt.Printf("Warning: NAT setup failed: %v\n", err)
	}

	return nil
}

func setupPfNat(natRule string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf(`echo '%s' | pfctl -ef -`, natRule))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pfctl: %w (%s)", err, string(out))
	}
	return nil
}

func (t *TUN) Read(buf []byte) (int, error) {
	return t.dev.Read(buf)
}

func (t *TUN) Write(buf []byte) (int, error) {
	return t.dev.Write(buf)
}

func (t *TUN) Close() error {
	if !t.isNode {
		// Client: remove routes and restore DNS
		if runtime.GOOS == "darwin" {
			exec.Command("route", "delete", "-net", "0.0.0.0/1").Run()
			exec.Command("route", "delete", "-net", "128.0.0.0/1").Run()
			exec.Command("route", "delete", "-host", t.nodeIP).Run()
			t.restoreDNSDarwin()
		} else {
			exec.Command("ip", "route", "del", "0.0.0.0/1", "dev", t.name).Run()
			exec.Command("ip", "route", "del", "128.0.0.0/1", "dev", t.name).Run()
			exec.Command("ip", "route", "del", t.nodeIP+"/32").Run()
		}
	} else {
		// Node: cleanup NAT and routes
		if runtime.GOOS == "linux" {
			exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", t.subnet, "-j", "MASQUERADE").Run()
			exec.Command("ip", "route", "del", t.subnet, "dev", t.name).Run()
		} else if runtime.GOOS == "darwin" {
			exec.Command("pfctl", "-d").Run()
			exec.Command("route", "delete", "-net", t.subnet).Run()
		}
	}

	return t.dev.Close()
}

func (t *TUN) Name() string {
	return t.name
}

// getSubnetBase returns base of subnet (e.g., "11.0.0.0/24" -> "11.0.0")
func getSubnetBase(subnet string) string {
	parts := strings.Split(subnet, "/")
	if len(parts) == 0 {
		return subnet
	}
	ipParts := strings.Split(parts[0], ".")
	if len(ipParts) >= 3 {
		return strings.Join(ipParts[:3], ".")
	}
	return parts[0]
}

// getFirstClientIP returns first usable client IP (e.g., "11.0.0.0/24" -> "11.0.0.2")
func getFirstClientIP(subnet string) string {
	base := getSubnetBase(subnet)
	return base + ".2"
}
