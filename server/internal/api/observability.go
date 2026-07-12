package api

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the Prometheus metrics wired into the control plane (OTA-034).
type Metrics struct {
	gatherer prometheus.Gatherer

	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	InFlightGauge   prometheus.Gauge
}

// NewMetrics creates the OTA-034 Prometheus metric set and registers it with
// reg. When reg is nil the process-default registry is used (production);
// callers that need isolation (tests) pass prometheus.NewRegistry().
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	g, _ := reg.(prometheus.Gatherer)
	m := &Metrics{
		gatherer: g,
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_ota_http_requests_total",
				Help: "Total number of HTTP requests processed, partitioned by method, route, and status code.",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "helix_ota_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds, partitioned by method and route.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		InFlightGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "helix_ota_http_requests_in_flight",
				Help: "Current number of in-flight HTTP requests being processed.",
			},
		),
	}
	reg.MustRegister(m.RequestsTotal, m.RequestDuration, m.InFlightGauge)
	return m
}

// Handler returns a Gin handler that serves Prometheus metrics from the
// registered gatherer at GET /metrics.
func (m *Metrics) Handler() gin.HandlerFunc {
	h := promhttp.HandlerFor(m.gatherer, promhttp.HandlerOpts{})
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// Middleware returns a Gin middleware that records request count, duration, and
// in-flight concurrency for every HTTP request processed by the router.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		m.InFlightGauge.Inc()

		c.Next()

		m.InFlightGauge.Dec()
		dur := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		m.RequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		m.RequestDuration.WithLabelValues(c.Request.Method, path).Observe(dur)
	}
}

// --- Structured logging (OTA-034) ---

const ctxLogger = "helix.logger"

// StructuredLoggingMiddleware returns a Gin middleware that creates a
// per-request *slog.Logger carrying the request-id (set by requestIDMiddleware,
// which MUST run before this middleware) and stores it in the Gin context.
func StructuredLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetString(ctxRequestID)
		logger := slog.Default().With("request_id", reqID)
		c.Set(ctxLogger, logger)
		c.Next()
	}
}

// LoggerFrom returns the request-scoped *slog.Logger stored in the Gin context
// by StructuredLoggingMiddleware. If no logger is found (middleware not wired),
// the process-default logger is returned so callers never get a nil logger.
func LoggerFrom(c *gin.Context) *slog.Logger {
	if l, ok := c.Get(ctxLogger); ok {
		return l.(*slog.Logger)
	}
	return slog.Default()
}

// NewJSONLogger creates a slog.JSONHandler writing to os.Stderr at the given
// level. This is the canonical structured-logging setup for the server entry
// point (main.go).
func NewJSONLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
