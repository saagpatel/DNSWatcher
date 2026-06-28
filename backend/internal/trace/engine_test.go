package trace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"dnswatcher/backend/internal/contracts"
	"dnswatcher/backend/internal/policy"
	"dnswatcher/backend/internal/testkit"
	"dnswatcher/backend/internal/trace"

	"github.com/miekg/dns"
)

func TestTraceHandlesReferralCNAMEAndNODATA(t *testing.T) {
	root := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "root.local.",
		FakeIP: "203.0.113.10",
		Zone:   ".",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Ns = []dns.RR{
				mustRR(t, "com. 300 IN NS ns.com.local."),
			}
			msg.Extra = []dns.RR{
				mustRR(t, "ns.com.local. 300 IN A 203.0.113.20"),
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer root.Shutdown()

	tld := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "ns.com.local.",
		FakeIP: "203.0.113.20",
		Zone:   "com.",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			switch r.Question[0].Name {
			case "example.com.", "www.example.com.", "nodata.example.com.":
				msg.Ns = []dns.RR{
					mustRR(t, "example.com. 300 IN NS ns1.example.com."),
				}
				msg.Extra = []dns.RR{
					mustRR(t, "ns1.example.com. 300 IN A 203.0.113.30"),
				}
			default:
				msg.Rcode = dns.RcodeNameError
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer tld.Shutdown()

	auth := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "ns1.example.com.",
		FakeIP: "203.0.113.30",
		Zone:   "example.com.",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			switch r.Question[0].Name {
			case "example.com.":
				if r.Question[0].Qtype == dns.TypeA {
					msg.Answer = []dns.RR{mustRR(t, "example.com. 300 IN A 93.184.216.34")}
					msg.Authoritative = true
				} else {
					msg.Authoritative = true
					msg.Ns = []dns.RR{mustRR(t, "example.com. 300 IN SOA ns1.example.com. hostmaster.example.com. 1 3600 600 86400 300")}
				}
			case "www.example.com.":
				msg.Answer = []dns.RR{mustRR(t, "www.example.com. 60 IN CNAME example.com.")}
				msg.Authoritative = true
			case "nodata.example.com.":
				msg.Authoritative = true
				msg.Ns = []dns.RR{mustRR(t, "nodata.example.com. 300 IN SOA ns1.example.com. hostmaster.example.com. 1 3600 600 86400 300")}
			default:
				msg.Authoritative = true
				msg.Rcode = dns.RcodeNameError
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer auth.Shutdown()

	endpoints := map[string]string{
		root.FakeIP: root.Endpoint,
		tld.FakeIP:  tld.Endpoint,
		auth.FakeIP: auth.Endpoint,
	}

	engine := trace.NewEngine(trace.Config{
		PerHopTimeout:      200 * time.Millisecond,
		OverallTimeout:     2 * time.Second,
		MaxDepth:           10,
		MaxUpstreamQueries: 20,
		Roots: []trace.ServerCandidate{
			trace.NewServerCandidate(root.Name, root.FakeIP, root.Zone, root.Endpoint),
		},
		DestinationPolicy: policy.AllowAllPolicy{},
		EndpointResolver: func(ip string) string {
			if endpoint, ok := endpoints[ip]; ok {
				return endpoint
			}
			return net.JoinHostPort(ip, "53")
		},
	})

	success, err := engine.Trace(context.Background(), contracts.TraceRequest{Domain: "example.com", QType: "A"})
	if err != nil {
		t.Fatalf("trace success: %v", err)
	}
	if success.FinalOutcome.Kind != "success" || len(success.Hops) != 3 {
		t.Fatalf("unexpected success trace: %+v", success.FinalOutcome)
	}
	assertNoTerminalHopPurpose(t, success.Hops)
	if success.Hops[success.FinalOutcome.TerminalHopIndex].HopPurpose != "delegation" {
		t.Fatalf("expected terminal success hop to keep delegation purpose, got %q", success.Hops[success.FinalOutcome.TerminalHopIndex].HopPurpose)
	}
	assertTraceJSONUsesArrays(t, success)

	cname, err := engine.Trace(context.Background(), contracts.TraceRequest{Domain: "www.example.com", QType: "A"})
	if err != nil {
		t.Fatalf("trace cname: %v", err)
	}
	foundCNAME := false
	for _, hop := range cname.Hops {
		if hop.ResponseKind == "cname" {
			foundCNAME = true
		}
	}
	if !foundCNAME {
		t.Fatalf("expected cname hop, got %+v", cname.Hops)
	}
	assertNoTerminalHopPurpose(t, cname.Hops)
	foundCNAMEFollow := false
	for _, hop := range cname.Hops {
		if hop.HopPurpose == "cname_follow" {
			foundCNAMEFollow = true
		}
	}
	if !foundCNAMEFollow {
		t.Fatalf("expected at least one CNAME restart hop to use cname_follow purpose, got %+v", cname.Hops)
	}

	nodata, err := engine.Trace(context.Background(), contracts.TraceRequest{Domain: "nodata.example.com", QType: "AAAA"})
	if err != nil {
		t.Fatalf("trace nodata: %v", err)
	}
	if nodata.FinalOutcome.Kind != "success" {
		t.Fatalf("expected nodata success, got %+v", nodata.FinalOutcome)
	}
	lastHop := nodata.Hops[len(nodata.Hops)-1]
	if lastHop.ResponseKind != "nodata" {
		t.Fatalf("expected nodata response kind, got %s", lastHop.ResponseKind)
	}
	assertNoTerminalHopPurpose(t, nodata.Hops)
	if lastHop.HopPurpose != "delegation" {
		t.Fatalf("expected terminal NODATA hop to keep delegation purpose, got %q", lastHop.HopPurpose)
	}
}

func TestTraceHandlesSupportLookupAndTCPFallback(t *testing.T) {
	root := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "root.local.",
		FakeIP: "203.0.113.10",
		Zone:   ".",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			switch r.Question[0].Name {
			case "service.example.com.":
				msg.Ns = []dns.RR{mustRR(t, "com. 300 IN NS ns.com.local.")}
				msg.Extra = []dns.RR{mustRR(t, "ns.com.local. 300 IN A 203.0.113.20")}
			default:
				msg.Ns = []dns.RR{mustRR(t, "net. 300 IN NS ns.net.local.")}
				msg.Extra = []dns.RR{mustRR(t, "ns.net.local. 300 IN A 203.0.113.40")}
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer root.Shutdown()

	comServer := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "ns.com.local.",
		FakeIP: "203.0.113.20",
		Zone:   "com.",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Ns = []dns.RR{mustRR(t, "example.com. 300 IN NS ns.outside.net.")}
			_ = w.WriteMsg(msg)
		}),
	})
	defer comServer.Shutdown()

	netServer := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "ns.net.local.",
		FakeIP: "203.0.113.40",
		Zone:   "net.",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Ns = []dns.RR{mustRR(t, "outside.net. 300 IN NS ns-auth.outside.net.")}
			msg.Extra = []dns.RR{mustRR(t, "ns-auth.outside.net. 300 IN A 203.0.113.50")}
			_ = w.WriteMsg(msg)
		}),
	})
	defer netServer.Shutdown()

	outsideAuth := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "ns-auth.outside.net.",
		FakeIP: "203.0.113.50",
		Zone:   "outside.net.",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			if r.Question[0].Name == "ns.outside.net." {
				msg.Answer = []dns.RR{mustRR(t, "ns.outside.net. 300 IN A 203.0.113.60")}
				msg.Authoritative = true
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer outsideAuth.Shutdown()

	targetAuth := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "ns.outside.net.",
		FakeIP: "203.0.113.60",
		Zone:   "example.com.",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Authoritative = true
			if w.LocalAddr().Network() == "tcp" {
				msg.Answer = []dns.RR{mustRR(t, "service.example.com. 300 IN A 203.0.113.99")}
			} else {
				msg.Truncated = true
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer targetAuth.Shutdown()

	endpoints := map[string]string{
		root.FakeIP:        root.Endpoint,
		comServer.FakeIP:   comServer.Endpoint,
		netServer.FakeIP:   netServer.Endpoint,
		outsideAuth.FakeIP: outsideAuth.Endpoint,
		targetAuth.FakeIP:  targetAuth.Endpoint,
	}

	engine := trace.NewEngine(trace.Config{
		PerHopTimeout:      200 * time.Millisecond,
		OverallTimeout:     2 * time.Second,
		MaxDepth:           20,
		MaxUpstreamQueries: 40,
		Roots: []trace.ServerCandidate{
			trace.NewServerCandidate(root.Name, root.FakeIP, root.Zone, root.Endpoint),
		},
		DestinationPolicy: policy.AllowAllPolicy{},
		EndpointResolver: func(ip string) string {
			if endpoint, ok := endpoints[ip]; ok {
				return endpoint
			}
			return net.JoinHostPort(ip, "53")
		},
	})

	result, err := engine.Trace(context.Background(), contracts.TraceRequest{Domain: "service.example.com", QType: "A"})
	if err != nil {
		t.Fatalf("trace support lookup: %v", err)
	}
	if result.FinalOutcome.Kind != "success" {
		t.Fatalf("expected success, got %+v", result.FinalOutcome)
	}
	assertNoTerminalHopPurpose(t, result.Hops)
	foundSupportHop := false
	foundTCP := false
	for _, hop := range result.Hops {
		if hop.HopPurpose == "nameserver_address_lookup" {
			foundSupportHop = true
		}
		if hop.ServerIP == targetAuth.FakeIP && hop.Transport == "tcp" && hop.Truncated {
			foundTCP = true
		}
	}
	if !foundSupportHop {
		t.Fatalf("expected support lookup hop, got %+v", result.Hops)
	}
	if !foundTCP {
		t.Fatalf("expected tcp fallback hop, got %+v", result.Hops)
	}
}

func TestTraceClassifiesRefusedAndNotImplemented(t *testing.T) {
	root := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "root.local.",
		FakeIP: "1.1.1.1",
		Zone:   ".",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Ns = []dns.RR{
				mustRR(t, "example.com. 300 IN NS ns1.example.com."),
			}
			msg.Extra = []dns.RR{
				mustRR(t, "ns1.example.com. 300 IN A 1.1.1.2"),
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer root.Shutdown()

	auth := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "ns1.example.com.",
		FakeIP: "1.1.1.2",
		Zone:   "example.com.",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Authoritative = true
			switch r.Question[0].Name {
			case "refused.example.com.":
				msg.Rcode = dns.RcodeRefused
			case "notimp.example.com.":
				msg.Rcode = dns.RcodeNotImplemented
			default:
				msg.Rcode = dns.RcodeNameError
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer auth.Shutdown()

	endpoints := map[string]string{
		root.FakeIP: root.Endpoint,
		auth.FakeIP: auth.Endpoint,
	}

	engine := trace.NewEngine(trace.Config{
		PerHopTimeout:      200 * time.Millisecond,
		OverallTimeout:     2 * time.Second,
		MaxDepth:           10,
		MaxUpstreamQueries: 20,
		Roots: []trace.ServerCandidate{
			trace.NewServerCandidate(root.Name, root.FakeIP, root.Zone, root.Endpoint),
		},
		DestinationPolicy: policy.AllowAllPolicy{},
		EndpointResolver: func(ip string) string {
			if endpoint, ok := endpoints[ip]; ok {
				return endpoint
			}
			return net.JoinHostPort(ip, "53")
		},
	})

	refused, err := engine.Trace(context.Background(), contracts.TraceRequest{Domain: "refused.example.com", QType: "A"})
	if err != nil {
		t.Fatalf("trace refused: %v", err)
	}
	if refused.FinalOutcome.Kind != "refused" {
		t.Fatalf("expected refused outcome, got %+v", refused.FinalOutcome)
	}
	assertNoTerminalHopPurpose(t, refused.Hops)

	notImplemented, err := engine.Trace(context.Background(), contracts.TraceRequest{Domain: "notimp.example.com", QType: "A"})
	if err != nil {
		t.Fatalf("trace not implemented: %v", err)
	}
	if notImplemented.FinalOutcome.Kind != "not_implemented" {
		t.Fatalf("expected not_implemented outcome, got %+v", notImplemented.FinalOutcome)
	}
	assertNoTerminalHopPurpose(t, notImplemented.Hops)
}

func TestTraceStopsOnBlockedReferralDestination(t *testing.T) {
	root := testkit.StartServer(t, testkit.ServerSpec{
		Name:   "root.local.",
		FakeIP: "1.1.1.1",
		Zone:   ".",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Ns = []dns.RR{
				mustRR(t, "example.com. 300 IN NS ns1.example.com."),
			}
			msg.Extra = []dns.RR{
				mustRR(t, "ns1.example.com. 300 IN A 203.0.113.20"),
			}
			_ = w.WriteMsg(msg)
		}),
	})
	defer root.Shutdown()

	engine := trace.NewEngine(trace.Config{
		PerHopTimeout:      200 * time.Millisecond,
		OverallTimeout:     2 * time.Second,
		MaxDepth:           10,
		MaxUpstreamQueries: 20,
		Roots: []trace.ServerCandidate{
			trace.NewServerCandidate(root.Name, root.FakeIP, root.Zone, root.Endpoint),
		},
		DestinationPolicy: policy.PublicIPPolicy{},
		EndpointResolver: func(ip string) string {
			if ip == root.FakeIP {
				return root.Endpoint
			}
			return net.JoinHostPort(ip, "53")
		},
	})

	result, err := engine.Trace(context.Background(), contracts.TraceRequest{Domain: "blocked.example.com", QType: "A"})
	if err != nil {
		t.Fatalf("trace blocked destination: %v", err)
	}
	if result.FinalOutcome.Kind != "unusable_referral" {
		t.Fatalf("expected unusable_referral outcome, got %+v", result.FinalOutcome)
	}
	if len(result.Hops) == 0 || result.Hops[len(result.Hops)-1].ResponseKind != "error" {
		t.Fatalf("expected terminal error hop, got %+v", result.Hops)
	}
	assertNoTerminalHopPurpose(t, result.Hops)
}

func assertNoTerminalHopPurpose(t *testing.T, hops []contracts.Hop) {
	t.Helper()
	for _, hop := range hops {
		if hop.HopPurpose == "terminal" {
			t.Fatalf("hop %d used unsupported terminal hop_purpose: %+v", hop.Index, hop)
		}
	}
}

func assertTraceJSONUsesArrays(t *testing.T, result contracts.TraceResult) {
	t.Helper()
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal trace result: %v", err)
	}
	for _, fragment := range [][]byte{
		[]byte(`"answer_rrsets":null`),
		[]byte(`"authority_rrsets":null`),
		[]byte(`"additional_rrsets":null`),
		[]byte(`"next_targets":null`),
	} {
		if bytes.Contains(payload, fragment) {
			t.Fatalf("trace JSON contained null array field %s: %s", fragment, payload)
		}
	}
}

func mustRR(t *testing.T, record string) dns.RR {
	t.Helper()
	rr, err := dns.NewRR(record)
	if err != nil {
		t.Fatalf("new rr %q: %v", record, err)
	}
	return rr
}
