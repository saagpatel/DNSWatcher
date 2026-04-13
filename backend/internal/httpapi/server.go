package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"dnswatcher/backend/internal/contracts"
	"dnswatcher/backend/internal/policy"

	"golang.org/x/time/rate"
)

type Tracer interface {
	Trace(context.Context, contracts.TraceRequest) (contracts.TraceResult, error)
}

type Config struct {
	Logger              *slog.Logger
	BodyLimitBytes      int64
	RateLimitPerMinute  int
	Burst               int
	MaxConcurrentTraces int
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	MaxHeaderBytes      int
	StaticDir           string
}

type Server struct {
	tracer                 Tracer
	logger                 *slog.Logger
	bodyLimit              int64
	limiterMu              sync.Mutex
	limiters               map[string]*limiterEntry
	limiterRate            rate.Limit
	limiterBurst           int
	limiterTTL             time.Duration
	limiterCleanupInterval time.Duration
	lastLimiterCleanup     time.Time
	semaphore              chan struct{}
	staticDir              string
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewServer(tracer Tracer, cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	bodyLimit := cfg.BodyLimitBytes
	if bodyLimit == 0 {
		bodyLimit = 2048
	}
	perMinute := cfg.RateLimitPerMinute
	if perMinute == 0 {
		perMinute = 20
	}
	burst := cfg.Burst
	if burst == 0 {
		burst = 5
	}
	maxConcurrent := cfg.MaxConcurrentTraces
	if maxConcurrent == 0 {
		maxConcurrent = 8
	}
	return &Server{
		tracer:                 tracer,
		logger:                 logger,
		bodyLimit:              bodyLimit,
		limiters:               map[string]*limiterEntry{},
		limiterRate:            rate.Every(time.Minute / time.Duration(perMinute)),
		limiterBurst:           burst,
		limiterTTL:             15 * time.Minute,
		limiterCleanupInterval: 5 * time.Minute,
		lastLimiterCleanup:     time.Now(),
		semaphore:              make(chan struct{}, maxConcurrent),
		staticDir:              cfg.StaticDir,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/api/v1/traces", s.createTrace)
	if s.staticDir != "" {
		fileServer := http.FileServer(http.Dir(s.staticDir))
		mux.Handle("/", fileServer)
	}
	return withJSONHeaders(mux)
}

func StdlibServer(handler http.Handler, cfg Config) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: defaultDuration(cfg.ReadHeaderTimeout, 5*time.Second),
		ReadTimeout:       defaultDuration(cfg.ReadTimeout, 10*time.Second),
		WriteTimeout:      defaultDuration(cfg.WriteTimeout, 30*time.Second),
		IdleTimeout:       defaultDuration(cfg.IdleTimeout, 60*time.Second),
		MaxHeaderBytes:    defaultInt(cfg.MaxHeaderBytes, 1<<20),
	}
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) createTrace(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	clientKey := clientKeyFromRequest(r)
	clientLogKey := hashClientKey(clientKey)
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, contracts.ErrorResponse{Error: "method_not_allowed", Message: "Use POST /api/v1/traces."})
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, contracts.ErrorResponse{Error: "unsupported_media_type", Message: "Requests must use Content-Type: application/json."})
		return
	}
	limiter := s.limiterFor(clientKey)
	if !limiter.Allow() {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, contracts.ErrorResponse{Error: "rate_limited", Message: "Too many trace requests. Please retry later."})
		return
	}
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	default:
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusTooManyRequests, contracts.ErrorResponse{Error: "concurrency_limited", Message: "The service is busy. Please retry shortly."})
		return
	}

	body := http.MaxBytesReader(w, r.Body, s.bodyLimit)
	defer body.Close()

	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var req contracts.TraceRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, contracts.ErrorResponse{Error: "invalid_request", Message: "Request body must contain a valid domain and qtype."})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, contracts.ErrorResponse{Error: "invalid_request", Message: "Request body must contain a single JSON object."})
		return
	}

	result, err := s.tracer.Trace(r.Context(), req)
	if err != nil {
		s.writeTraceError(w, err)
		s.logger.Warn("trace_failed",
			slog.String("client", clientLogKey),
			slog.String("qtype", strings.ToUpper(req.QType)),
			slog.Duration("duration", time.Since(started)),
			slog.String("error", err.Error()),
		)
		return
	}

	s.logger.Info("trace_completed",
		slog.String("client", clientLogKey),
		slog.String("qtype", result.QType),
		slog.String("outcome", result.FinalOutcome.Kind),
		slog.Int("hop_count", len(result.Hops)),
		slog.Int("duration_ms", result.TotalDurationMS),
	)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) writeTraceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, policy.ErrInvalidDomain), errors.Is(err, policy.ErrUnsupportedQType), errors.Is(err, policy.ErrSpecialUseDomain), errors.Is(err, policy.ErrIPLiteralDomain), errors.Is(err, policy.ErrDomainTooManyLabels):
		writeJSON(w, http.StatusBadRequest, contracts.ErrorResponse{Error: "invalid_domain_input", Message: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, contracts.ErrorResponse{Error: "internal_error", Message: "The trace service encountered an unexpected error."})
	}
}

func (s *Server) limiterFor(key string) *rate.Limiter {
	now := time.Now()
	s.limiterMu.Lock()
	defer s.limiterMu.Unlock()

	if now.Sub(s.lastLimiterCleanup) >= s.limiterCleanupInterval {
		s.cleanupLimiters(now)
		s.lastLimiterCleanup = now
	}

	if entry, ok := s.limiters[key]; ok {
		entry.lastSeen = now
		return entry.limiter
	}

	limiter := rate.NewLimiter(s.limiterRate, s.limiterBurst)
	s.limiters[key] = &limiterEntry{limiter: limiter, lastSeen: now}
	return limiter
}

func (s *Server) cleanupLimiters(now time.Time) {
	for key, entry := range s.limiters {
		if now.Sub(entry.lastSeen) > s.limiterTTL {
			delete(s.limiters, key)
		}
	}
}

func withJSONHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func clientKeyFromRequest(r *http.Request) string {
	for _, header := range []string{"Fly-Client-IP", "CF-Connecting-IP", "X-Real-IP"} {
		if ip := canonicalClientIP(r.Header.Get(header)); ip != "" {
			return ip
		}
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		for _, part := range strings.Split(forwarded, ",") {
			if ip := canonicalClientIP(part); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		if ip := canonicalClientIP(r.RemoteAddr); ip != "" {
			return ip
		}
		return r.RemoteAddr
	}
	return host
}

func canonicalClientIP(value string) string {
	ip := strings.TrimSpace(value)
	if ip == "" {
		return ""
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	return addr.String()
}

func hashClientKey(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:6])
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
