package httpapi_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"dnswatcher/backend/internal/contracts"
	"dnswatcher/backend/internal/httpapi"
	"dnswatcher/backend/internal/policy"
)

type stubTracer struct {
	result contracts.TraceResult
	err    error
}

func (s stubTracer) Trace(context.Context, contracts.TraceRequest) (contracts.TraceResult, error) {
	return s.result, s.err
}

type blockingTracer struct {
	result  contracts.TraceResult
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingTracer) Trace(ctx context.Context, _ contracts.TraceRequest) (contracts.TraceResult, error) {
	s.once.Do(func() {
		close(s.started)
	})
	select {
	case <-s.release:
		return s.result, nil
	case <-ctx.Done():
		return contracts.TraceResult{}, ctx.Err()
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestCreateTraceRejectsInvalidMethodAndContentType(t *testing.T) {
	server := httpapi.NewServer(stubTracer{}, httpapi.Config{})
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"example.com","qtype":"A"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
}

func TestCreateTraceRejectsInvalidInputAndHonorsBodyLimit(t *testing.T) {
	server := httpapi.NewServer(stubTracer{err: policy.ErrInvalidDomain}, httpapi.Config{BodyLimitBytes: 16})
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"bad","qtype":"TXT"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/traces", bytes.NewBufferString(strings.Repeat("a", 64)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized malformed body, got %d", rec.Code)
	}
}

func TestCreateTraceRateLimitAndLoggingDoNotLeakDomain(t *testing.T) {
	result := contracts.TraceResult{QType: "A", FinalOutcome: contracts.FinalOutcome{Kind: "success"}, Hops: []contracts.Hop{{Index: 0}}, TotalDurationMS: 12}
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	server := httpapi.NewServer(stubTracer{result: result}, httpapi.Config{Logger: logger, RateLimitPerMinute: 1, Burst: 1})
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"secret.example.com","qtype":"A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"secret.example.com","qtype":"A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.1:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	if strings.Contains(logBuffer.String(), "secret.example.com") {
		t.Fatalf("expected logs to avoid raw domains, got %s", logBuffer.String())
	}
	if strings.Contains(logBuffer.String(), "192.0.2.1") {
		t.Fatalf("expected logs to avoid raw client addresses, got %s", logBuffer.String())
	}
}

func TestCreateTraceRateLimitUsesDirectPeerByDefault(t *testing.T) {
	result := contracts.TraceResult{QType: "A", FinalOutcome: contracts.FinalOutcome{Kind: "success"}, Hops: []contracts.Hop{{Index: 0}}, TotalDurationMS: 12}
	server := httpapi.NewServer(stubTracer{result: result}, httpapi.Config{Logger: silentLogger(), RateLimitPerMinute: 1, Burst: 1})
	handler := server.Handler()

	first := postTrace(t, handler, "203.0.113.10:1234", http.Header{"X-Forwarded-For": []string{"198.51.100.30"}})
	if first != http.StatusOK {
		t.Fatalf("expected 200 for first direct client, got %d", first)
	}
	repeat := postTrace(t, handler, "203.0.113.10:9999", http.Header{"Forwarded": []string{`for=198.51.100.31`}})
	if repeat != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when the same direct peer repeats, got %d", repeat)
	}
	other := postTrace(t, handler, "203.0.113.11:1234", http.Header{"X-Forwarded-For": []string{"198.51.100.30"}})
	if other != http.StatusOK {
		t.Fatalf("expected 200 for a different direct peer, got %d", other)
	}
}

func TestCreateTraceIgnoresSpoofedHeadersFromUntrustedPeers(t *testing.T) {
	result := contracts.TraceResult{QType: "A", FinalOutcome: contracts.FinalOutcome{Kind: "success"}, Hops: []contracts.Hop{{Index: 0}}, TotalDurationMS: 12}
	trusted, invalid := httpapi.ParseTrustedProxyCIDRs([]string{"10.0.0.0/8", "2001:db8:1::/64"})
	if len(invalid) != 0 {
		t.Fatalf("unexpected invalid CIDRs: %v", invalid)
	}
	server := httpapi.NewServer(stubTracer{result: result}, httpapi.Config{
		Logger:             silentLogger(),
		RateLimitPerMinute: 1,
		Burst:              1,
		TrustedProxyCIDRs:  trusted,
	})
	handler := server.Handler()

	headers := http.Header{
		"X-Forwarded-For":  []string{"198.51.100.30, 10.0.0.2"},
		"Forwarded":        []string{`for=198.51.100.30`},
		"Fly-Client-Ip":    []string{"198.51.100.30"},
		"Cf-Connecting-Ip": []string{"198.51.100.30"},
		"X-Real-Ip":        []string{"198.51.100.30"},
	}
	if got := postTrace(t, handler, "203.0.113.40:1234", headers); got != http.StatusOK {
		t.Fatalf("expected 200 for first untrusted peer, got %d", got)
	}
	if got := postTrace(t, handler, "203.0.113.41:1234", headers); got != http.StatusOK {
		t.Fatalf("expected 200 when a different untrusted peer spoofs the same forwarded client, got %d", got)
	}
	if got := postTrace(t, handler, "[2001:db8:9::40]:1234", headers); got != http.StatusOK {
		t.Fatalf("expected 200 for an untrusted ipv6 peer with spoofed headers, got %d", got)
	}
}

func TestCreateTraceTrustedProxyChainRateLimitsCanonicalClient(t *testing.T) {
	result := contracts.TraceResult{QType: "A", FinalOutcome: contracts.FinalOutcome{Kind: "success"}, Hops: []contracts.Hop{{Index: 0}}, TotalDurationMS: 12}
	trusted, invalid := httpapi.ParseTrustedProxyCIDRs([]string{"10.0.0.0/8", "2001:db8:1::/64"})
	if len(invalid) != 0 {
		t.Fatalf("unexpected invalid CIDRs: %v", invalid)
	}
	server := httpapi.NewServer(stubTracer{result: result}, httpapi.Config{
		Logger:             silentLogger(),
		RateLimitPerMinute: 1,
		Burst:              1,
		TrustedProxyCIDRs:  trusted,
	})
	handler := server.Handler()

	if got := postTrace(t, handler, "10.0.0.2:80", http.Header{"X-Forwarded-For": []string{"198.51.100.30, 10.0.0.8"}}); got != http.StatusOK {
		t.Fatalf("expected 200 for first trusted-proxy client, got %d", got)
	}
	if got := postTrace(t, handler, "10.0.0.9:80", http.Header{"X-Forwarded-For": []string{"198.51.100.30"}}); got != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when the same canonical client repeats through another trusted proxy, got %d", got)
	}
	if got := postTrace(t, handler, "[2001:db8:1::2]:80", http.Header{"Forwarded": []string{`for="[2001:db8:cafe::17]"`}}); got != http.StatusOK {
		t.Fatalf("expected 200 for a distinct ipv6 client behind a trusted proxy, got %d", got)
	}
	if got := postTrace(t, handler, "[2001:db8:1::9]:80", http.Header{
		"Forwarded": []string{`for="[2001:0db8:cafe:0000:0000:0000:0000:0017]"`},
	}); got != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for the same canonical ipv6 client through another trusted proxy, got %d", got)
	}
}

func TestCreateTraceMalformedForwardedHeadersStayOnTrustedPeer(t *testing.T) {
	result := contracts.TraceResult{QType: "A", FinalOutcome: contracts.FinalOutcome{Kind: "success"}, Hops: []contracts.Hop{{Index: 0}}, TotalDurationMS: 12}
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	trusted, invalid := httpapi.ParseTrustedProxyCIDRs([]string{"10.0.0.0/8"})
	if len(invalid) != 0 {
		t.Fatalf("unexpected invalid CIDRs: %v", invalid)
	}
	server := httpapi.NewServer(stubTracer{result: result}, httpapi.Config{
		Logger:             logger,
		RateLimitPerMinute: 1,
		Burst:              1,
		TrustedProxyCIDRs:  trusted,
	})
	handler := server.Handler()

	malformed := http.Header{
		"Forwarded":       []string{`for="_gazonk", for=unknown`, `for=not-an-ip`},
		"X-Forwarded-For": []string{"???, not-an-ip"},
	}
	if got := postTrace(t, handler, "10.0.0.2:80", malformed); got != http.StatusOK {
		t.Fatalf("expected 200 using the trusted peer after malformed headers, got %d", got)
	}
	if got := postTrace(t, handler, "10.0.0.2:81", http.Header{"X-Forwarded-For": []string{"198.51.100.99, garbage"}}); got != http.StatusOK {
		t.Fatalf("expected 200 for a usable client after skipping malformed xff hops, got %d", got)
	}
	if got := postTrace(t, handler, "10.0.0.2:82", malformed); got != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when malformed headers keep the shared trusted-peer key, got %d", got)
	}

	logs := logBuffer.String()
	if !strings.Contains(logs, `"forwarded_header_ignored"`) || !strings.Contains(logs, `"malformed"`) {
		t.Fatalf("expected malformed forwarded headers to be recorded, got %s", logs)
	}
	if strings.Contains(logs, "10.0.0.2") || strings.Contains(logs, "198.51.100.99") || strings.Contains(logs, "_gazonk") {
		t.Fatalf("expected logs to avoid raw addresses and header values, got %s", logs)
	}
}

func postTrace(t *testing.T, handler http.Handler, remoteAddr string, header http.Header) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"example.com","qtype":"A"}`))
	req.Header.Set("Content-Type", "application/json")
	for key, values := range header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

func TestCreateTraceConcurrencyLimit(t *testing.T) {
	tracer := &blockingTracer{
		result:  contracts.TraceResult{QType: "A", FinalOutcome: contracts.FinalOutcome{Kind: "success"}, Hops: []contracts.Hop{{Index: 0}}, TotalDurationMS: 12},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server := httpapi.NewServer(tracer, httpapi.Config{RateLimitPerMinute: 60, Burst: 60, MaxConcurrentTraces: 1})
	handler := server.Handler()

	firstDone := make(chan int, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"example.com","qtype":"A"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		firstDone <- rec.Code
	}()

	<-tracer.started

	req := httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"example.com","qtype":"A"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 while one trace is in flight, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "concurrency_limited") {
		t.Fatalf("expected concurrency_limited response, got %s", rec.Body.String())
	}

	close(tracer.release)
	if code := <-firstDone; code != http.StatusOK {
		t.Fatalf("expected first request to finish 200, got %d", code)
	}
}
