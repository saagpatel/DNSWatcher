package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const (
	clientSourceRemoteAddr    = "remote_addr"
	clientSourceForwarded     = "forwarded"
	clientSourceXForwardedFor = "x_forwarded_for"

	forwardedDispositionAbsent               = "absent"
	forwardedDispositionHonored              = "honored"
	forwardedDispositionIgnoredUntrustedPeer = "ignored_untrusted_peer"
	forwardedDispositionIgnoredMalformed     = "ignored_malformed"
	forwardedDispositionIgnoredConflict      = "ignored_conflict"
)

// clientIdentity is the canonical rate-limit key derived from a request.
// Address is a stable netip-canonical IP string when parseable.
type clientIdentity struct {
	Address       string
	Source        string
	Disposition   string
	IgnoredReason string
}

// ParseTrustedProxyCIDRs parses configured proxy networks. Bare IPs are
// accepted as single-host prefixes. Invalid values are returned separately so
// callers can fail closed at process startup.
func ParseTrustedProxyCIDRs(values []string) (prefixes []netip.Prefix, invalid []string) {
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			addr, addrErr := netip.ParseAddr(raw)
			if addrErr != nil {
				invalid = append(invalid, raw)
				continue
			}
			addr = addr.Unmap()
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, invalid
}

func resolveClientIdentity(remoteAddr string, header http.Header, trusted []netip.Prefix) clientIdentity {
	peer, peerKey := peerIdentity(remoteAddr)
	forwardedPresent := hasForwardedHeaders(header)
	if !forwardedPresent {
		return clientIdentity{
			Address:     peerKey,
			Source:      clientSourceRemoteAddr,
			Disposition: forwardedDispositionAbsent,
		}
	}

	if !peer.IsValid() || !addrIsTrusted(peer, trusted) {
		return clientIdentity{
			Address:       peerKey,
			Source:        clientSourceRemoteAddr,
			Disposition:   forwardedDispositionIgnoredUntrustedPeer,
			IgnoredReason: "untrusted_peer",
		}
	}

	forwardedHops, _ := parseForwardedForAddrs(header.Values("Forwarded"))
	xffHops, _ := parseXForwardedForAddrs(header.Values("X-Forwarded-For"))
	forwardedClient, forwardedOK := clientFromTrustedChain(forwardedHops, trusted)
	xffClient, xffOK := clientFromTrustedChain(xffHops, trusted)

	switch {
	case forwardedOK && xffOK && forwardedClient.Compare(xffClient) != 0:
		return clientIdentity{
			Address:       peerKey,
			Source:        clientSourceRemoteAddr,
			Disposition:   forwardedDispositionIgnoredConflict,
			IgnoredReason: "conflicting_forwarded_headers",
		}
	case xffOK:
		return clientIdentity{
			Address:     canonicalAddr(xffClient),
			Source:      clientSourceXForwardedFor,
			Disposition: forwardedDispositionHonored,
		}
	case forwardedOK:
		return clientIdentity{
			Address:     canonicalAddr(forwardedClient),
			Source:      clientSourceForwarded,
			Disposition: forwardedDispositionHonored,
		}
	default:
		return clientIdentity{
			Address:       peerKey,
			Source:        clientSourceRemoteAddr,
			Disposition:   forwardedDispositionIgnoredMalformed,
			IgnoredReason: "malformed",
		}
	}
}

func hasForwardedHeaders(header http.Header) bool {
	return len(header.Values("Forwarded")) > 0 || len(header.Values("X-Forwarded-For")) > 0
}

func peerIdentity(remoteAddr string) (netip.Addr, string) {
	host := remoteAddr
	if parsedHost, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = parsedHost
	}
	if addr, ok := parseIPHop(host); ok {
		return addr, canonicalAddr(addr)
	}
	if remoteAddr == "" {
		return netip.Addr{}, ""
	}
	return netip.Addr{}, remoteAddr
}

func clientFromTrustedChain(hops []netip.Addr, trusted []netip.Prefix) (netip.Addr, bool) {
	if len(hops) == 0 {
		return netip.Addr{}, false
	}
	for i := len(hops) - 1; i >= 0; i-- {
		if !addrIsTrusted(hops[i], trusted) {
			return hops[i], true
		}
	}
	return hops[0], true
}

func addrIsTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	if !addr.IsValid() || len(trusted) == 0 {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseXForwardedForAddrs(values []string) ([]netip.Addr, bool) {
	var hops []netip.Addr
	malformed := false
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			addr, ok := parseIPHop(part)
			if !ok {
				malformed = true
				continue
			}
			hops = append(hops, addr)
		}
	}
	return hops, malformed
}

func parseForwardedForAddrs(values []string) ([]netip.Addr, bool) {
	var hops []netip.Addr
	malformed := false
	for _, value := range values {
		for _, element := range splitIgnoringQuotes(value, ',') {
			element = strings.TrimSpace(element)
			if element == "" {
				continue
			}
			forParam, found := forwardedForParam(element)
			if !found {
				malformed = true
				continue
			}
			addr, ok := parseIPHop(forParam)
			if !ok {
				malformed = true
				continue
			}
			hops = append(hops, addr)
		}
	}
	return hops, malformed
}

func forwardedForParam(element string) (string, bool) {
	for _, pair := range splitIgnoringQuotes(element, ';') {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(key), "for") {
			continue
		}
		return unquoteForwardedValue(strings.TrimSpace(value)), true
	}
	return "", false
}

func unquoteForwardedValue(value string) string {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	inner := value[1 : len(value)-1]
	var b strings.Builder
	b.Grow(len(inner))
	escaped := false
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func splitIgnoringQuotes(value string, sep byte) []string {
	var parts []string
	var b strings.Builder
	inQuotes := false
	escaped := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if inQuotes && c == '\\' {
			b.WriteByte(c)
			escaped = true
			continue
		}
		if c == '"' {
			inQuotes = !inQuotes
			b.WriteByte(c)
			continue
		}
		if c == sep && !inQuotes {
			parts = append(parts, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	parts = append(parts, b.String())
	return parts
}

func parseIPHop(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	if value == "" || strings.EqualFold(value, "unknown") || strings.HasPrefix(value, "_") {
		return netip.Addr{}, false
	}
	if strings.HasPrefix(value, "[") {
		end := strings.IndexByte(value, ']')
		if end < 0 {
			return netip.Addr{}, false
		}
		host := value[1:end]
		rest := value[end+1:]
		if rest != "" && !strings.HasPrefix(rest, ":") {
			return netip.Addr{}, false
		}
		return parseCanonicalAddr(host)
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return parseCanonicalAddr(host)
	}
	return parseCanonicalAddr(value)
}

func parseCanonicalAddr(value string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func canonicalAddr(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.Unmap().String()
}
