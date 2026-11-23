package checker

import "net/netip"

var blockedPrefixes = []netip.Prefix{
	// IPv4: shared, documentation, benchmarking and reserved ranges.
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),

	// IPv6: documentation and transition ranges.
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func isPublicIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}

	// IPv4-mapped IPv6 addresses must be checked as IPv4.
	ip = ip.Unmap()

	if ip.Zone() != "" {
		return false
	}

	if !ip.IsGlobalUnicast() ||
		ip.IsPrivate() ||
		ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() {
		return false
	}

	// Only accept IPv6 global unicast allocation.
	if ip.Is6() {
		globalIPv6 := netip.MustParsePrefix("2000::/3")
		if !globalIPv6.Contains(ip) {
			return false
		}
	}

	for _, prefix := range blockedPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}

	return true
}
