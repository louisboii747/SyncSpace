package discovery

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

type networkSnapshot struct {
	fingerprint          string
	primaryIP            string
	interfaceName        string
	interfaceDescription string
	selectionReason      string
}

type interfaceAddress struct {
	index       int
	name        string
	description string
	flags       net.Flags
	ip          net.IP
}

type selectedInterface struct {
	interfaceAddress
	score  int
	reason string
}

func inspectNetwork() (networkSnapshot, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return networkSnapshot{}, fmt.Errorf("list network interfaces: %w", err)
	}

	descriptions := loadInterfaceDescriptions()
	candidates := make([]interfaceAddress, 0)
	for _, networkInterface := range interfaces {
		interfaceAddresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range interfaceAddresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil {
				continue
			}
			candidates = append(candidates, interfaceAddress{
				index:       networkInterface.Index,
				name:        networkInterface.Name,
				description: descriptions[networkInterface.Index],
				flags:       networkInterface.Flags,
				ip:          ip,
			})
		}
	}

	selection, eligible := selectPreferredInterface(candidates)
	if selection == nil {
		return networkSnapshot{}, nil
	}
	keys := make([]string, 0, len(eligible))
	for _, candidate := range eligible {
		keys = append(keys, fmt.Sprintf("%d:%s:%s", candidate.index, candidate.name, candidate.ip.String()))
	}
	sort.Strings(keys)
	return networkSnapshot{
		fingerprint:          strings.Join(keys, "|"),
		primaryIP:            selection.ip.String(),
		interfaceName:        selection.name,
		interfaceDescription: selection.description,
		selectionReason:      selection.reason,
	}, nil
}

func selectPreferredInterface(candidates []interfaceAddress) (*selectedInterface, []interfaceAddress) {
	eligible := make([]interfaceAddress, 0, len(candidates))
	ranked := make([]selectedInterface, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.flags&net.FlagUp == 0 || candidate.flags&net.FlagLoopback != 0 ||
			isObviousVirtualInterface(interfaceIdentity(candidate)) || !usableLocalIP(candidate.ip) {
			continue
		}
		eligible = append(eligible, candidate)
		score, reason := scoreInterface(candidate)
		ranked = append(ranked, selectedInterface{interfaceAddress: candidate, score: score, reason: reason})
	}
	if len(ranked) == 0 {
		return nil, eligible
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		leftName := strings.ToLower(ranked[i].name)
		rightName := strings.ToLower(ranked[j].name)
		if leftName != rightName {
			return leftName < rightName
		}
		return ranked[i].ip.String() < ranked[j].ip.String()
	})
	return &ranked[0], eligible
}

func scoreInterface(candidate interfaceAddress) (int, string) {
	score := 0
	reasons := make([]string, 0, 3)
	if isPhysicalLANInterface(interfaceIdentity(candidate)) {
		score += 1000
		reasons = append(reasons, "active Wi-Fi/Ethernet interface")
	} else {
		reasons = append(reasons, "active non-virtual interface")
	}

	switch privateIPv4Range(candidate.ip) {
	case "192.168/16":
		score += 300
		reasons = append(reasons, "preferred 192.168/16 private IPv4")
	case "10/8":
		score += 250
		reasons = append(reasons, "preferred 10/8 private IPv4")
	case "172.16/12":
		score += 200
		reasons = append(reasons, "non-virtual 172.16/12 private IPv4")
	default:
		if candidate.ip.To4() != nil {
			score += 100
			reasons = append(reasons, "usable IPv4 fallback")
		} else {
			score += 20
			reasons = append(reasons, "usable IPv6 fallback")
		}
	}
	if candidate.flags&net.FlagMulticast != 0 {
		score += 5
	}
	return score, strings.Join(reasons, "; ")
}

func interfaceIdentity(candidate interfaceAddress) string {
	return strings.TrimSpace(candidate.name + " " + candidate.description)
}

func privateIPv4Range(ip net.IP) string {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return ""
	}
	switch {
	case ipv4[0] == 192 && ipv4[1] == 168:
		return "192.168/16"
	case ipv4[0] == 10:
		return "10/8"
	case ipv4[0] == 172 && ipv4[1] >= 16 && ipv4[1] <= 31:
		return "172.16/12"
	default:
		return ""
	}
}

func isPhysicalLANInterface(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, marker := range []string{"wi-fi", "wifi", "wireless", "wlan", "ethernet", "local area connection"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	for _, prefix := range []string{"eth", "en", "wl"} {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func isObviousVirtualInterface(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	markers := []string{
		"wsl", "hyper-v", "vethernet", "veth", "docker", "container",
		"vmware", "virtualbox", "vbox", "tailscale", "loopback",
		"bluetooth", "teredo", "wireguard", "openvpn", "tap-windows",
		"utun", "virbr", "vpn",
	}
	for _, marker := range markers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return strings.HasPrefix(normalized, "wg") || strings.HasPrefix(normalized, "tun") ||
		strings.HasPrefix(normalized, "tap") || strings.HasPrefix(normalized, "br-")
}

func usableLocalIP(ip net.IP) bool {
	return ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast()
}
