package checker

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"testing"
)

type fakeResolver struct {
	ips   []netip.Addr
	err   error
	calls int
}

func (r *fakeResolver) LookupNetIP(
	_ context.Context,
	_ string,
	_ string,
) ([]netip.Addr, error) {
	r.calls++
	return r.ips, r.err
}

func TestSafeDialerConnectsToResolvedIP(t *testing.T) {
	resolver := &fakeResolver{
		ips: []netip.Addr{
			netip.MustParseAddr("8.8.8.8"),
		},
	}

	var dialedAddress string

	d := &safeDialer{
		resolver: resolver,
		isPublicIP: func(ip netip.Addr) bool {
			return isPublicIP(ip)
		},
		dial: func(
			_ context.Context,
			network string,
			address string,
		) (net.Conn, error) {
			dialedAddress = address

			client, server := net.Pipe()
			_ = server.Close()

			return client, nil
		},
	}

	conn, err := d.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	if dialedAddress != "8.8.8.8:443" {
		t.Errorf(
			"dialed address = %q, want %q",
			dialedAddress,
			"8.8.8.8:443",
		)
	}

	if resolver.calls != 1 {
		t.Errorf("resolver calls = %d, want 1", resolver.calls)
	}
}

func TestSafeDialerRejectsNonPublicIP(t *testing.T) {
	resolver := &fakeResolver{
		ips: []netip.Addr{
			netip.MustParseAddr("8.8.8.8"),
			netip.MustParseAddr("127.0.0.1"),
		},
	}

	dialCalled := false

	d := &safeDialer{
		resolver: resolver,
		isPublicIP: func(ip netip.Addr) bool {
			return isPublicIP(ip)
		},
		dial: func(
			context.Context,
			string,
			string,
		) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("unexpected dial")
		},
	}

	_, err := d.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)

	if err == nil {
		t.Fatal("expected error for non-public IP")
	}

	if dialCalled {
		t.Fatal("dial was called for a non-public destination")
	}
}

func TestSafeDialerReturnsResolverError(t *testing.T) {
	resolver := &fakeResolver{
		err: errors.New("DNS unavailable"),
	}

	d := &safeDialer{
		resolver: resolver,
		isPublicIP: func(netip.Addr) bool {
			return true
		},
		dial: func(
			context.Context,
			string,
			string,
		) (net.Conn, error) {
			t.Fatal("dial must not be called")
			return nil, nil
		},
	}

	_, err := d.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)

	if err == nil {
		t.Fatal("expected DNS error")
	}
}

func TestSafeDialerRejectsEmptyDNSResult(t *testing.T) {
	d := &safeDialer{
		resolver: &fakeResolver{},
		isPublicIP: func(netip.Addr) bool {
			return true
		},
		dial: func(
			context.Context,
			string,
			string,
		) (net.Conn, error) {
			t.Fatal("dial must not be called")
			return nil, nil
		},
	}

	_, err := d.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)

	if err == nil {
		t.Fatal("expected error for empty DNS result")
	}
}

func TestSafeDialerUsesIPLiteralWithoutDNS(t *testing.T) {
	resolver := &fakeResolver{}

	var dialedAddress string

	d := &safeDialer{
		resolver: resolver,
		isPublicIP: func(ip netip.Addr) bool {
			return isPublicIP(ip)
		},
		dial: func(
			_ context.Context,
			_ string,
			address string,
		) (net.Conn, error) {
			dialedAddress = address

			client, server := net.Pipe()
			_ = server.Close()

			return client, nil
		},
	}

	conn, err := d.DialContext(
		context.Background(),
		"tcp",
		"8.8.8.8:443",
	)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	if resolver.calls != 0 {
		t.Errorf("resolver calls = %d, want 0", resolver.calls)
	}

	if dialedAddress != "8.8.8.8:443" {
		t.Errorf("dialed address = %q", dialedAddress)
	}
}

func TestSafeDialerTriesOnlyValidatedIPs(t *testing.T) {
	resolver := &fakeResolver{
		ips: []netip.Addr{
			netip.MustParseAddr("8.8.8.8"),
			netip.MustParseAddr("1.1.1.1"),
		},
	}

	var dialedAddresses []string

	d := &safeDialer{
		resolver: resolver,
		isPublicIP: func(ip netip.Addr) bool {
			return isPublicIP(ip)
		},
		dial: func(
			_ context.Context,
			_ string,
			address string,
		) (net.Conn, error) {
			dialedAddresses = append(dialedAddresses, address)

			if address == "8.8.8.8:443" {
				return nil, errors.New("connection failed")
			}

			client, server := net.Pipe()
			_ = server.Close()

			return client, nil
		},
	}

	conn, err := d.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	want := []string{
		"8.8.8.8:443",
		"1.1.1.1:443",
	}

	if !reflect.DeepEqual(dialedAddresses, want) {
		t.Errorf(
			"dialed addresses = %v, want %v",
			dialedAddresses,
			want,
		)
	}
}
