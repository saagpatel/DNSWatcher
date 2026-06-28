package httpapi_test

import (
	"bytes"
	"context"
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

func TestCreateTraceUsesForwardedClientIPForRateLimiting(t *testing.T) {
	result := contracts.TraceResult{QType: "A", FinalOutcome: contracts.FinalOutcome{Kind: "success"}, Hops: []contracts.Hop{{Index: 0}}, TotalDurationMS: 12}
	server := httpapi.NewServer(stubTracer{result: result}, httpapi.Config{RateLimitPerMinute: 1, Burst: 1})
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"example.com","qtype":"A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "198.51.100.30, 10.0.0.2")
	req.RemoteAddr = "100.64.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/traces", strings.NewReader(`{"domain":"example.com","qtype":"A"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "198.51.100.30, 10.0.0.2")
	req.RemoteAddr = "100.64.0.8:9999"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when forwarded client repeats, got %d", rec.Code)
	}
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
