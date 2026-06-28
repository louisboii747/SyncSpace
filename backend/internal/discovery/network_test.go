package discovery

import (
	"net"
	"testing"
)

func TestSelectPreferredInterface(t *testing.T) {
	active := net.FlagUp | net.FlagMulticast
	tests := []struct {
		name       string
		candidates []interfaceAddress
		wantName   string
		wantIP     string
	}{
		{
			name: "physical Wi-Fi beats WSL adapter",
			candidates: []interfaceAddress{
				{name: "vEthernet (WSL)", flags: active, ip: net.ParseIP("172.28.48.1")},
				{name: "Wi-Fi", flags: active, ip: net.ParseIP("192.168.1.42")},
			},
			wantName: "Wi-Fi",
			wantIP:   "192.168.1.42",
		},
		{
			name: "physical Ethernet on 172 range beats Docker",
			candidates: []interfaceAddress{
				{name: "docker0", flags: active, ip: net.ParseIP("10.0.0.1")},
				{name: "Ethernet", flags: active, ip: net.ParseIP("172.20.10.4")},
			},
			wantName: "Ethernet",
			wantIP:   "172.20.10.4",
		},
		{
			name: "physical interface priority precedes private range priority",
			candidates: []interfaceAddress{
				{name: "adapter0", flags: active, ip: net.ParseIP("192.168.50.2")},
				{name: "en0", flags: active, ip: net.ParseIP("10.20.30.40")},
			},
			wantName: "en0",
			wantIP:   "10.20.30.40",
		},
		{
			name: "link local and down adapters are ignored",
			candidates: []interfaceAddress{
				{name: "Wi-Fi", flags: active, ip: net.ParseIP("169.254.5.5")},
				{name: "Ethernet", flags: 0, ip: net.ParseIP("192.168.1.9")},
				{name: "wlan0", flags: active, ip: net.ParseIP("10.1.2.3")},
			},
			wantName: "wlan0",
			wantIP:   "10.1.2.3",
		},
		{
			name: "public IPv4 remains a last resort",
			candidates: []interfaceAddress{
				{name: "Ethernet", flags: active, ip: net.ParseIP("203.0.113.10")},
				{name: "Tailscale", flags: active, ip: net.ParseIP("100.64.0.2")},
			},
			wantName: "Ethernet",
			wantIP:   "203.0.113.10",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selected, _ := selectPreferredInterface(test.candidates)
			if selected == nil {
				t.Fatal("expected an interface selection")
			}
			if selected.name != test.wantName || selected.ip.String() != test.wantIP {
				t.Fatalf("selected %s/%s, want %s/%s", selected.name, selected.ip, test.wantName, test.wantIP)
			}
			if selected.reason == "" {
				t.Fatal("selection reason was empty")
			}
		})
	}
}

func TestSelectPreferredInterfaceRejectsVirtualAdapters(t *testing.T) {
	active := net.FlagUp | net.FlagMulticast
	virtualNames := []string{
		"vEthernet (Default Switch)", "WSL", "Docker Desktop", "VMware Network Adapter",
		"VirtualBox Host-Only", "Tailscale", "Bluetooth PAN", "Teredo Tunneling",
		"WireGuard Tunnel", "OpenVPN TAP-Windows6", "utun4", "br-a12b3c",
	}
	for _, name := range virtualNames {
		t.Run(name, func(t *testing.T) {
			selected, eligible := selectPreferredInterface([]interfaceAddress{{
				name: name, flags: active, ip: net.ParseIP("192.168.1.5"),
			}})
			if selected != nil || len(eligible) != 0 {
				t.Fatalf("virtual adapter %q was not rejected", name)
			}
		})
	}
}

func TestSelectPreferredInterfaceUsesWindowsAdapterDescription(t *testing.T) {
	active := net.FlagUp | net.FlagMulticast
	selected, _ := selectPreferredInterface([]interfaceAddress{
		{
			name:        "Ethernet 3",
			description: "VirtualBox Host-Only Ethernet Adapter",
			flags:       active,
			ip:          net.ParseIP("192.168.56.1"),
		},
		{
			name:        "Wi-Fi",
			description: "Intel(R) Wi-Fi 6 AX201",
			flags:       active,
			ip:          net.ParseIP("192.168.1.42"),
		},
	})
	if selected == nil || selected.name != "Wi-Fi" {
		t.Fatalf("selected %#v, want physical Wi-Fi", selected)
	}
}

func TestBestAddressPrefersLANRangesOverRecordOrder(t *testing.T) {
	selected := bestAddress(
		[]net.IP{net.ParseIP("172.28.48.1"), net.ParseIP("10.1.2.3"), net.ParseIP("192.168.1.42")},
		nil,
	)
	if selected == nil || selected.String() != "192.168.1.42" {
		t.Fatalf("selected %v, want 192.168.1.42", selected)
	}
}
