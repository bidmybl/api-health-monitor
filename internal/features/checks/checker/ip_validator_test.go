package checker

import (
	"net/netip"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"public IPv4", "8.8.8.8", true},
		{"public IPv4 Cloudflare", "1.1.1.1", true},
		{"public IPv6", "2606:4700:4700::1111", true},

		{"loopback IPv4", "127.0.0.1", false},
		{"loopback IPv6", "::1", false},
		{"private IPv4", "192.168.1.1", false},
		{"private 10/8", "10.0.0.1", false},
		{"private IPv6", "fd00::1", false},
		{"link-local IPv4", "169.254.1.1", false},
		{"link-local IPv6", "fe80::1", false},
		{"unspecified IPv4", "0.0.0.0", false},
		{"unspecified IPv6", "::", false},
		{"multicast IPv4", "224.0.0.1", false},
		{"documentation IPv4", "192.0.2.1", false},
		{"documentation IPv6", "2001:db8::1", false},
		{"IPv4-mapped loopback", "::ffff:127.0.0.1", false},
		{"invalid IP", "not-an-ip", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := netip.ParseAddr(tt.ip)
			if err != nil && tt.want {
				t.Fatalf("ParseAddr(%q): %v", tt.ip, err)
			}

			if got := isPublicIP(ip); got != tt.want {
				t.Errorf("isPublicIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
