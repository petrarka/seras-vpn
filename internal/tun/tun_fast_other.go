//go:build !linux

package tun

// FastTUN wraps TUN (no io_uring on non-Linux)
type FastTUN struct {
	*TUN
}

// NewFast creates a TUN (no io_uring on this platform)
func NewFast(localIP, gateway, nodeIP, nodeVPNIP string) (*FastTUN, error) {
	return NewFastWithDNS(localIP, gateway, nodeIP, nodeVPNIP, []string{"8.8.8.8", "1.1.1.1"})
}

// NewFastWithDNS creates a TUN with custom DNS
func NewFastWithDNS(localIP, gateway, nodeIP, nodeVPNIP string, dnsServers []string) (*FastTUN, error) {
	t, err := NewWithDNS(localIP, gateway, nodeIP, nodeVPNIP, dnsServers)
	if err != nil {
		return nil, err
	}
	return &FastTUN{TUN: t}, nil
}

// NewFastNode creates a node TUN
func NewFastNode(localIP, vpnSubnet string) (*FastTUN, error) {
	t, err := NewNodeTUN(localIP, vpnSubnet)
	if err != nil {
		return nil, err
	}
	return &FastTUN{TUN: t}, nil
}

// HasIOURing returns false on non-Linux
func (t *FastTUN) HasIOURing() bool {
	return false
}
