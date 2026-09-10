package httpapi

import (
	"net/http"
	"net/netip"
	"testing"
)

func TestParseTrustedProxyCIDRs(t *testing.T) {
	prefixes, invalid := ParseTrustedProxyCIDRs([]string{
		"10.0.0.0/8",
		" 192.0.2.64 ",
		"2001:db8:1::/64",
		"not-a-cidr",
		"",
		"::ffff:198.51.100.8",
	})
	if len(invalid) != 1 || invalid[0] != "not-a-cidr" {
		t.Fatalf("expected invalid not-a-cidr, got %#v", invalid)
	}
	if len(prefixes) != 4 {
		t.Fatalf("expected 4 prefixes, got %d", len(prefixes))
	}
	if !prefixes[1].Contains(netip.MustParseAddr("192.0.2.64")) || prefixes[1].Bits() != 32 {
		t.Fatalf("expected bare IPv4 to become /32, got %s", prefixes[1])
	}
	if !prefixes[3].Contains(netip.MustParseAddr("198.51.100.8")) {
		t.Fatalf("expected unmapped IPv4-mapped bare IP to match 198.51.100.8, got %s", prefixes[3])
	}
}

func TestResolveClientIdentity(t *testing.T) {
	trusted, invalid := ParseTrustedProxyCIDRs([]string{"10.0.0.0/8", "192.0.2.64/32", "2001:db8:1::/64"})
	if len(invalid) != 0 {
		t.Fatalf("unexpected invalid CIDRs: %v", invalid)
	}

	tests := []struct {
		name            string
		remoteAddr      string
		header          http.Header
		trusted         []netip.Prefix
		wantAddress     string
		wantSource      string
		wantReason      string
		wantDisposition string
	}{
		{
			name:            "direct ipv4 peer without forwarded headers",
			remoteAddr:      "203.0.113.10:443",
			wantAddress:     "203.0.113.10",
			wantSource:      clientSourceRemoteAddr,
			wantDisposition: forwardedDispositionAbsent,
		},
		{
			name:            "direct ipv6 peer without forwarded headers",
			remoteAddr:      "[2001:db8:9::10]:443",
			wantAddress:     "2001:db8:9::10",
			wantSource:      clientSourceRemoteAddr,
			wantDisposition: forwardedDispositionAbsent,
		},
		{
			name:       "spoofed xff from untrusted ipv4 peer is ignored",
			remoteAddr: "203.0.113.20:1234",
			header: http.Header{
				"X-Forwarded-For": []string{"198.51.100.30, 10.0.0.2"},
			},
			trusted:         trusted,
			wantAddress:     "203.0.113.20",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "untrusted_peer",
			wantDisposition: forwardedDispositionIgnoredUntrustedPeer,
		},
		{
			name:       "spoofed forwarded from untrusted ipv6 peer is ignored",
			remoteAddr: "[2001:db8:9::20]:1234",
			header: http.Header{
				"Forwarded": []string{`for="[2001:db8:cafe::17]"`},
			},
			trusted:         trusted,
			wantAddress:     "2001:db8:9::20",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "untrusted_peer",
			wantDisposition: forwardedDispositionIgnoredUntrustedPeer,
		},
		{
			name:       "vendor headers are never a client source",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Fly-Client-Ip":    []string{"198.51.100.99"},
				"Cf-Connecting-Ip": []string{"198.51.100.99"},
				"X-Real-Ip":        []string{"198.51.100.99"},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantDisposition: forwardedDispositionAbsent,
		},
		{
			name:       "vendor client headers from untrusted peers are ignored",
			remoteAddr: "203.0.113.21:9",
			header: http.Header{
				"Fly-Client-Ip":    []string{"198.51.100.99"},
				"Cf-Connecting-Ip": []string{"198.51.100.99"},
				"X-Real-Ip":        []string{"198.51.100.99"},
				"X-Forwarded-For":  []string{"198.51.100.99"},
			},
			trusted:         trusted,
			wantAddress:     "203.0.113.21",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "untrusted_peer",
			wantDisposition: forwardedDispositionIgnoredUntrustedPeer,
		},
		{
			name:       "trusted ipv4 proxy uses original client from xff chain",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"198.51.100.30, 10.0.0.8"},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.30",
			wantSource:      clientSourceXForwardedFor,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "trusted proxy chain skips spoofed leftmost xff hop",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"203.0.113.1, 198.51.100.40, 10.0.0.8"},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.40",
			wantSource:      clientSourceXForwardedFor,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "trusted ipv6 proxy uses forwarded for parameter",
			remoteAddr: "[2001:db8:1::2]:80",
			header: http.Header{
				"Forwarded": []string{`for="[2001:db8:cafe::17]"`},
			},
			trusted:         trusted,
			wantAddress:     "2001:db8:cafe::17",
			wantSource:      clientSourceForwarded,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "multiple forwarded headers append a trusted chain",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{
					`for=198.51.100.50`,
					`for=10.0.0.8;proto=https`,
				},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.50",
			wantSource:      clientSourceForwarded,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "comma-separated forwarded values walk from the right",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for=198.51.100.51, for="[2001:db8:cafe::51]", for=10.0.0.9`},
			},
			trusted:         trusted,
			wantAddress:     "2001:db8:cafe::51",
			wantSource:      clientSourceForwarded,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "multiple x-forwarded-for header values are one chain",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"198.51.100.60", "10.0.0.8"},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.60",
			wantSource:      clientSourceXForwardedFor,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "malformed xff hops are skipped and usable ipv4 remains",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"not-an-ip, unknown, 198.51.100.70, _hidden"},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.70",
			wantSource:      clientSourceXForwardedFor,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "malformed forwarded values fall back to the trusted peer",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for="_gazonk"`, `by=10.0.0.2;proto=https`, `for=unknown`},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "malformed",
			wantDisposition: forwardedDispositionIgnoredMalformed,
		},
		{
			name:       "malformed xff with no usable hop uses the trusted peer",
			remoteAddr: "192.0.2.64:80",
			header: http.Header{
				"X-Forwarded-For": []string{"???, 10.0.0.notanip"},
			},
			trusted:         trusted,
			wantAddress:     "192.0.2.64",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "malformed",
			wantDisposition: forwardedDispositionIgnoredMalformed,
		},
		{
			name:       "xff ipv4-mapped ipv6 canonicalizes to ipv4",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"::ffff:198.51.100.80"},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.80",
			wantSource:      clientSourceXForwardedFor,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "expanded ipv6 notation is a stable compressed key",
			remoteAddr: "[2001:db8:1::2]:80",
			header: http.Header{
				"X-Forwarded-For": []string{"2001:0db8:cafe:0000:0000:0000:0000:0090"},
			},
			trusted:         trusted,
			wantAddress:     "2001:db8:cafe::90",
			wantSource:      clientSourceXForwardedFor,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "bracketed ipv6 xff with port is accepted",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"[2001:db8:cafe::91]:62744"},
			},
			trusted:         trusted,
			wantAddress:     "2001:db8:cafe::91",
			wantSource:      clientSourceXForwardedFor,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "conflicting forwarded headers keep the trusted peer",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded":       []string{`for=198.51.100.1`},
				"X-Forwarded-For": []string{"198.51.100.2"},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "conflicting_forwarded_headers",
			wantDisposition: forwardedDispositionIgnoredConflict,
		},
		{
			name:       "matching forwarded and xff still honor the xff client",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded":       []string{`for=198.51.100.3`},
				"X-Forwarded-For": []string{"198.51.100.3, 10.0.0.8"},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.3",
			wantSource:      clientSourceXForwardedFor,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:            "empty trusted proxy list never honors forwarded headers",
			remoteAddr:      "10.0.0.2:80",
			header:          http.Header{"X-Forwarded-For": []string{"198.51.100.4"}},
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "untrusted_peer",
			wantDisposition: forwardedDispositionIgnoredUntrustedPeer,
		},
		{
			name:            "unparseable remote addr is used as a stable fallback key",
			remoteAddr:      "unix-socket",
			wantAddress:     "unix-socket",
			wantSource:      clientSourceRemoteAddr,
			wantDisposition: forwardedDispositionAbsent,
		},
		{
			name:       "valid quoted forwarded ipv4 is still honored",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for="198.51.100.1"`},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.1",
			wantSource:      clientSourceForwarded,
			wantDisposition: forwardedDispositionHonored,
		},
		{
			name:       "unbalanced opening forwarded quote is rejected",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for="198.51.100.1`},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "malformed",
			wantDisposition: forwardedDispositionIgnoredMalformed,
		},
		{
			name:       "unbalanced closing forwarded quote is rejected",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for=198.51.100.1"`},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "malformed",
			wantDisposition: forwardedDispositionIgnoredMalformed,
		},
		{
			name:       "doubled forwarded quotes are rejected",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for=""198.51.100.1""`},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "malformed",
			wantDisposition: forwardedDispositionIgnoredMalformed,
		},
		{
			name:       "trailing extra forwarded quote is rejected",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for="198.51.100.1""`},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "malformed",
			wantDisposition: forwardedDispositionIgnoredMalformed,
		},
		{
			name:       "quoted forwarded value with trailing escape is rejected",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for="198.51.100.1\`},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "malformed",
			wantDisposition: forwardedDispositionIgnoredMalformed,
		},
		{
			name:       "quoted xff hops are not stripped into client ips",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{`"198.51.100.1`, `"198.51.100.2"`},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "malformed",
			wantDisposition: forwardedDispositionIgnoredMalformed,
		},
		{
			name:       "all-trusted xff chain falls back to the direct peer",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"10.0.0.8, 10.0.0.9"},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "all_trusted_hops",
			wantDisposition: forwardedDispositionIgnoredAllTrusted,
		},
		{
			name:       "all-trusted forwarded chain falls back to the direct peer",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"Forwarded": []string{`for=10.0.0.8, for=10.0.0.9`},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "all_trusted_hops",
			wantDisposition: forwardedDispositionIgnoredAllTrusted,
		},
		{
			name:       "all-trusted ipv6 xff chain falls back to the direct peer",
			remoteAddr: "[2001:db8:1::2]:80",
			header: http.Header{
				"X-Forwarded-For": []string{"2001:db8:1::8, 2001:db8:1::9"},
			},
			trusted:         trusted,
			wantAddress:     "2001:db8:1::2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "all_trusted_hops",
			wantDisposition: forwardedDispositionIgnoredAllTrusted,
		},
		{
			name:       "single trusted xff hop cannot select a different trusted bucket",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"10.0.0.8"},
			},
			trusted:         trusted,
			wantAddress:     "10.0.0.2",
			wantSource:      clientSourceRemoteAddr,
			wantReason:      "all_trusted_hops",
			wantDisposition: forwardedDispositionIgnoredAllTrusted,
		},
		{
			name:       "all-trusted xff still allows a usable forwarded client",
			remoteAddr: "10.0.0.2:80",
			header: http.Header{
				"X-Forwarded-For": []string{"10.0.0.8, 10.0.0.9"},
				"Forwarded":       []string{`for=198.51.100.7`},
			},
			trusted:         trusted,
			wantAddress:     "198.51.100.7",
			wantSource:      clientSourceForwarded,
			wantDisposition: forwardedDispositionHonored,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveClientIdentity(tt.remoteAddr, tt.header, tt.trusted)
			if got.Address != tt.wantAddress {
				t.Fatalf("address: got %q, want %q", got.Address, tt.wantAddress)
			}
			if got.Source != tt.wantSource {
				t.Fatalf("source: got %q, want %q", got.Source, tt.wantSource)
			}
			if got.IgnoredReason != tt.wantReason {
				t.Fatalf("ignored reason: got %q, want %q", got.IgnoredReason, tt.wantReason)
			}
			if got.Disposition != tt.wantDisposition {
				t.Fatalf("disposition: got %q, want %q", got.Disposition, tt.wantDisposition)
			}
		})
	}
}

func TestCanonicalAddrStability(t *testing.T) {
	left := resolveClientIdentity("[2001:db8::1]:1", nil, nil)
	right := resolveClientIdentity("[2001:0db8:0000:0000:0000:0000:0000:0001]:9", nil, nil)
	if left.Address != "2001:db8::1" || left.Address != right.Address {
		t.Fatalf("expected stable ipv6 key 2001:db8::1, got %q and %q", left.Address, right.Address)
	}

	mapped := resolveClientIdentity("[::ffff:192.0.2.9]:1", nil, nil)
	v4 := resolveClientIdentity("192.0.2.9:8", nil, nil)
	if mapped.Address != "192.0.2.9" || mapped.Address != v4.Address {
		t.Fatalf("expected ipv4-mapped key 192.0.2.9, got %q and %q", mapped.Address, v4.Address)
	}
}

func TestParseQuotedStringRejectsMalformedFraming(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
		ok    bool
	}{
		{name: "balanced ipv4", value: `"198.51.100.1"`, want: "198.51.100.1", ok: true},
		{name: "balanced ipv6", value: `"[2001:db8::1]"`, want: "[2001:db8::1]", ok: true},
		{name: "escaped inner quote", value: `"198.51.100.1\""`, want: `198.51.100.1"`, ok: true},
		{name: "unbalanced opening", value: `"198.51.100.1`, ok: false},
		{name: "unbalanced closing", value: `198.51.100.1"`, ok: false},
		{name: "doubled quotes", value: `""198.51.100.1""`, ok: false},
		{name: "trailing extra quote", value: `"198.51.100.1""`, ok: false},
		{name: "trailing backslash", value: `"198.51.100.1\`, ok: false},
		{name: "empty quotes", value: `""`, want: "", ok: true},
		{name: "unquoted token", value: "198.51.100.1", want: "198.51.100.1", ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := unquoteForwardedValue(tt.value)
			if ok != tt.ok {
				t.Fatalf("ok: got %v, want %v (value %q -> %q)", ok, tt.ok, tt.value, got)
			}
			if tt.ok && got != tt.want {
				t.Fatalf("value: got %q, want %q", got, tt.want)
			}
		})
	}
}
