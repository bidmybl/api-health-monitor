package checker

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
)

type ipResolver interface {
	LookupNetIP(
		ctx context.Context,
		network string,
		host string,
	) ([]netip.Addr, error)
}

type safeDialer struct {
	resolver   ipResolver
	dial       func(context.Context, string, string) (net.Conn, error)
	isPublicIP func(netip.Addr) bool
}

func newSafeDialer() *safeDialer {
	d := &net.Dialer{}

	return &safeDialer{
		resolver:   net.DefaultResolver,
		dial:       d.DialContext,
		isPublicIP: isPublicIP,
	}
}

func (d *safeDialer) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split address: %w", err)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf("invalid port")
	}

	ips, err := d.resolve(ctx, host)
	if err != nil {
		return nil, err
	}

	for _, ip := range ips {
		if !d.isPublicIP(ip) {
			return nil, fmt.Errorf("destination IP is not public")
		}
	}

	var lastErr error

	for _, ip := range ips {
		conn, err := d.dial(
			ctx,
			network,
			net.JoinHostPort(ip.String(), port),
		)
		if err == nil {
			return conn, nil
		}

		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("connect to resolved IPs: %w", lastErr)
	}

	return nil, fmt.Errorf("DNS returned no IP addresses")
}

func (d *safeDialer) resolve(
	ctx context.Context,
	host string,
) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip.Unmap()}, nil
	}

	ips, err := d.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("host has no IP addresses")
	}

	result := make([]netip.Addr, 0, len(ips))
	seen := make(map[netip.Addr]struct{})

	for _, ip := range ips {
		ip = ip.Unmap()

		if _, exists := seen[ip]; exists {
			continue
		}

		seen[ip] = struct{}{}
		result = append(result, ip)
	}

	return result, nil
}
