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
	forwardedDispositionIgnoredAllTrusted    = "ignored_all_trusted"
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
	forwardedClient, forwardedOK, forwardedAllTrusted := clientFromTrustedChain(forwardedHops, trusted)
	xffClient, xffOK, xffAllTrusted := clientFromTrustedChain(xffHops, trusted)

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
	case forwardedAllTrusted || xffAllTrusted:
		return clientIdentity{
			Address:       peerKey,
			Source:        clientSourceRemoteAddr,
			Disposition:   forwardedDispositionIgnoredAllTrusted,
			IgnoredReason: "all_trusted_hops",
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

func clientFromTrustedChain(hops []netip.Addr, trusted []netip.Prefix) (netip.Addr, bool, bool) {
	if len(hops) == 0 {
		return netip.Addr{}, false, false
	}
	for i := len(hops) - 1; i >= 0; i-- {
		if !addrIsTrusted(hops[i], trusted) {
			return hops[i], true, false
		}
	}
	return netip.Addr{}, false, true
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
		unquoted, valid := unquoteForwardedValue(strings.TrimSpace(value))
		if !valid {
			return "", false
		}
		return unquoted, true
	}
	return "", false
}

func unquoteForwardedValue(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	if value[0] != '"' {
		if strings.ContainsRune(value, '"') {
			return "", false
		}
		return value, true
	}
	return parseQuotedString(value)
}

func parseQuotedString(value string) (string, bool) {
	if len(value) < 2 || value[0] != '"' {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(value))
	escaped := false
	closed := false
	for i := 1; i < len(value); i++ {
		c := value[i]
		if closed {
			return "", false
		}
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			closed = true
			continue
		}
		b.WriteByte(c)
	}
	if escaped || !closed {
		return "", false
	}
	return b.String(), true
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
	if value == "" || strings.ContainsRune(value, '"') || strings.EqualFold(value, "unknown") || strings.HasPrefix(value, "_") {
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
