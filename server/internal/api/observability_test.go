package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsEndpoint(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	r := gin.New()
	r.Use(m.Middleware())
	r.GET("/metrics", m.Handler())
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /metrics: want 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type: want text/plain, got %q", ct)
	}
	body := w.Body.String()
	for _, name := range []string{"helix_ota_http_requests_total", "helix_ota_http_request_duration_seconds", "helix_ota_http_requests_in_flight"} {
		if !strings.Contains(body, name) {
			t.Errorf("missing metric %q", name)
		}
	}
}

func TestMetricsAntiTautology(t *testing.T) {
	reg := prometheus.NewRegistry()
	rt := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "helix_ota_http_requests_total", Help: "Total."}, []string{"method", "path", "status"})
	rd := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "helix_ota_http_request_duration_seconds", Help: "Dur.", Buckets: prometheus.DefBuckets}, []string{"method", "path"})
	reg.MustRegister(rt, rd)
	m := &Metrics{gatherer: reg, RequestsTotal: rt, RequestDuration: rd}
	r := gin.New()
	r.GET("/metrics", m.Handler())
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "helix_ota_http_requests_in_flight") {
		t.Fatalf("RED: InFlightGauge absent from register — must NOT appear")
	}
	inFlight := prometheus.NewGauge(prometheus.GaugeOpts{Name: "helix_ota_http_requests_in_flight", Help: "In-flight."})
	reg.MustRegister(inFlight)
	m.InFlightGauge = inFlight
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "helix_ota_http_requests_in_flight") {
		t.Fatalf("GREEN: InFlightGauge registered — MUST appear")
	}
}

func TestStructuredLogging(t *testing.T) {
	r := gin.New()
	r.Use(requestIDMiddleware(), StructuredLoggingMiddleware())
	var capturedReqID string
	r.GET("/test", func(c *gin.Context) {
		if LoggerFrom(c) == nil { t.Error("LoggerFrom nil") }
		capturedReqID = c.GetString(ctxRequestID)
		c.String(http.StatusOK, "ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-Id", "test-trace-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 { t.Fatalf("want 200") }
	if capturedReqID != "test-trace-123" { t.Errorf("reqID mismatch") }
	if w.Header().Get("X-Request-Id") != "test-trace-123" { t.Errorf("header mismatch") }
}

func TestStructuredLoggingAutoGenRequestID(t *testing.T) {
	r := gin.New()
	r.Use(requestIDMiddleware(), StructuredLoggingMiddleware())
	var capturedReqID string
	r.GET("/test", func(c *gin.Context) {
		if LoggerFrom(c) == nil { t.Error("LoggerFrom nil") }
		capturedReqID = c.GetString(ctxRequestID)
		c.String(http.StatusOK, "ok")
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	if w.Code != 200 { t.Fatalf("want 200") }
	if capturedReqID == "" { t.Error("auto-gen reqID empty") }
}
