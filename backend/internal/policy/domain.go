package policy

import (
	"errors"
	"net/netip"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidDomain      = errors.New("invalid domain input")
	ErrUnsupportedQType   = errors.New("unsupported query type")
	ErrSpecialUseDomain   = errors.New("special-use domain is not allowed")
	ErrIPLiteralDomain    = errors.New("ip literals are not allowed")
	ErrDomainTooManyLabels = errors.New("domain has too many labels")
)

var supportedQTypes = map[string]struct{}{
	"A":    {},
	"AAAA": {},
	"NS":   {},
}

var blockedDomains = []string{
	"localhost",
	"local",
	"home.arpa",
	"invalid",
	"test",
}

func NormalizeDomain(input string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(input))
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return "", ErrInvalidDomain
	}
	if !utf8.ValidString(domain) {
		return "", ErrInvalidDomain
	}
	for _, r := range domain {
		if r > 127 {
			return "", ErrInvalidDomain
		}
	}
	if ip, err := netip.ParseAddr(domain); err == nil && ip.IsValid() {
		return "", ErrIPLiteralDomain
	}
	labels := strings.Split(domain, ".")
	if len(labels) > 20 {
		return "", ErrDomainTooManyLabels
	}
	if len(domain) > 253 {
		return "", ErrInvalidDomain
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return "", ErrInvalidDomain
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrInvalidDomain
		}
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return "", ErrInvalidDomain
			}
		}
	}
	for _, blocked := range blockedDomains {
		if domain == blocked || strings.HasSuffix(domain, "."+blocked) {
			return "", ErrSpecialUseDomain
		}
	}
	if strings.HasSuffix(domain, ".arpa") {
		if strings.HasSuffix(domain, ".in-addr.arpa") || strings.HasSuffix(domain, ".ip6.arpa") {
			return "", ErrSpecialUseDomain
		}
	}
	return domain, nil
}

func NormalizeQType(input string) (string, error) {
	qtype := strings.ToUpper(strings.TrimSpace(input))
	if _, ok := supportedQTypes[qtype]; !ok {
		return "", ErrUnsupportedQType
	}
	return qtype, nil
}
