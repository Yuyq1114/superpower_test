package nginx_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// normalizeResolverAddress is a pure-Go behavioral mirror of the
// normalize_ipv4/normalize_ipv6 shell functions in start-nginx.sh. It exists
// so the edge cases below (valid/invalid IPv4, bare/scoped IPv6, and
// injection attempts) can be exercised on any host OS, including Windows,
// which has no guaranteed POSIX sh/awk. It is not the authoritative
// implementation: the real shell script is separately verified for
// structural equivalence by TestStartScriptValidatesStrictIPv4Octets /
// TestStartScriptValidatesIPv6WithScopeStrippingAndBrackets above, and
// exercised for real inside Linux containers by the Docker resolver smoke
// tests recorded in the Task 8 report.
func normalizeResolverAddress(raw string) (string, error) {
	if strings.Contains(raw, ":") {
		return normalizeIPv6Mirror(raw)
	}
	return normalizeIPv4Mirror(raw)
}

func normalizeIPv4Mirror(addr string) (string, error) {
	octets := strings.Split(addr, ".")
	if len(octets) != 4 {
		return "", fmt.Errorf("invalid IPv4 nameserver address: %s", addr)
	}
	for _, octet := range octets {
		if octet == "" || len(octet) > 3 {
			return "", fmt.Errorf("invalid IPv4 nameserver address: %s", addr)
		}
		for _, c := range octet {
			if c < '0' || c > '9' {
				return "", fmt.Errorf("invalid IPv4 nameserver address: %s", addr)
			}
		}
		n, err := strconv.Atoi(octet)
		if err != nil || n > 255 {
			return "", fmt.Errorf("invalid IPv4 nameserver address: %s", addr)
		}
	}
	return addr, nil
}

func normalizeIPv6Mirror(addr string) (string, error) {
	body := addr
	if idx := strings.Index(addr, "%"); idx >= 0 {
		body = addr[:idx]
	}
	if body == "" {
		return "", fmt.Errorf("invalid IPv6 nameserver address: %s", addr)
	}
	for _, c := range body {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		case c == ':' || c == '.':
		default:
			return "", fmt.Errorf("invalid IPv6 nameserver address: %s", addr)
		}
	}
	return "[" + body + "]", nil
}

func TestNormalizeResolverAddressMirror(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"plain ipv4", "8.8.8.8", "8.8.8.8", false},
		{"docker embedded resolver", "127.0.0.11", "127.0.0.11", false},
		{"ipv4 max octets", "255.255.255.255", "255.255.255.255", false},
		{"ipv4 too few octets", "1.2.3", "", true},
		{"ipv4 too many octets", "1.2.3.4.5", "", true},
		{"ipv4 octet out of range", "1.2.3.256", "", true},
		{"ipv4 non numeric octet", "1.2.3.4a", "", true},
		{"ipv4 cidr suffix rejected", "127.0.0.1/24", "", true},
		{"ipv4 empty octet", "1..3.4", "", true},
		{"bare ipv6", "2001:db8::1", "[2001:db8::1]", false},
		{"loopback ipv6", "::1", "[::1]", false},
		{"scoped ipv6 strips zone name", "fe80::1%eth0", "[fe80::1]", false},
		{"scoped ipv6 strips numeric zone", "fe80::1%25", "[fe80::1]", false},
		{"ipv6 with embedded ipv4 tail", "::ffff:192.0.2.1", "[::ffff:192.0.2.1]", false},
		{"ipv6 rejects cidr slash", "2001:db8::1/64", "", true},
		{"ipv6 rejects pipe injection", "2001:db8::1|touch /tmp/pwned", "", true},
		{"ipv6 rejects ampersand injection", "2001:db8::1&reboot", "", true},
		{"ipv4 rejects shell injection", "127.0.0.1;rm -rf /", "", true},
		{"zone-only value has no colon, dispatched as ipv4, invalid", "%eth0", "", true},
		{"empty value", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeResolverAddress(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeResolverAddress(%q) = %q, want error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeResolverAddress(%q) unexpected error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeResolverAddress(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
