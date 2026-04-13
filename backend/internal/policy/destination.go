package policy

import (
	"errors"
	"net/netip"
)

var ErrBlockedDestination = errors.New("destination ip is not allowed")

type DestinationPolicy interface {
	Allow(ip string) error
}

type PublicIPPolicy struct{}

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func (PublicIPPolicy) Allow(ip string) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ErrBlockedDestination
	}
	if !addr.IsGlobalUnicast() || addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return ErrBlockedDestination
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return ErrBlockedDestination
		}
	}
	return nil
}

type AllowAllPolicy struct{}

func (AllowAllPolicy) Allow(string) error {
	return nil
}
