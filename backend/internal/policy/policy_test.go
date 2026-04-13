package policy_test

import (
	"strings"
	"testing"

	"dnswatcher/backend/internal/policy"
)

func TestNormalizeDomainRejectsUnsafeInputs(t *testing.T) {
	tooManyLabels := strings.Repeat("a.", 20) + "com"
	cases := []struct {
		name   string
		input  string
		target error
	}{
		{name: "ip literal", input: "192.0.2.5", target: policy.ErrIPLiteralDomain},
		{name: "special use", input: "router.home.arpa", target: policy.ErrSpecialUseDomain},
		{name: "too many labels", input: tooManyLabels, target: policy.ErrDomainTooManyLabels},
		{name: "bad characters", input: "bad_domain.example.com", target: policy.ErrInvalidDomain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := policy.NormalizeDomain(tc.input)
			if err != tc.target {
				t.Fatalf("expected %v, got %v", tc.target, err)
			}
		})
	}
}

func TestNormalizeDomainAcceptsCanonicalInput(t *testing.T) {
	domain, err := policy.NormalizeDomain(" Example.COM. ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != "example.com" {
		t.Fatalf("expected normalized domain, got %q", domain)
	}
}

func TestPublicIPPolicyBlocksSpecialPurposeRanges(t *testing.T) {
	policy := policy.PublicIPPolicy{}
	blocked := []string{"10.0.0.1", "127.0.0.1", "192.0.2.10", "2001:db8::1"}
	for _, ip := range blocked {
		if err := policy.Allow(ip); err == nil {
			t.Fatalf("expected %s to be blocked", ip)
		}
	}
	if err := policy.Allow("8.8.8.8"); err != nil {
		t.Fatalf("expected public ip to be allowed: %v", err)
	}
}
