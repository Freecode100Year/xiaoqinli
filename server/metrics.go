package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector holds all Prometheus metrics for the server.
type MetricsCollector struct {
	// Decode metrics
	decodeTotal     *prometheus.CounterVec
	decodeFailTotal *prometheus.CounterVec

	// MCP tools call metrics
	toolsCallTotal   *prometheus.CounterVec
	toolsCallFail    *prometheus.CounterVec
	toolsCallDuration *prometheus.HistogramVec

	// Compile metrics
	compileTotal    *prometheus.CounterVec
	compileFail     *prometheus.CounterVec
	compileDuration *prometheus.HistogramVec

	registry *prometheus.Registry
}

// NewMetricsCollector creates a new metrics collector with Prometheus standard library.
func NewMetricsCollector() *MetricsCollector {
	registry := prometheus.NewRegistry()

	m := &MetricsCollector{
		registry: registry,
	}

	// Decode metrics
	m.decodeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xqlb_decode_total",
			Help: "Total number of XQLB decode operations",
		},
		[]string{"result"},
	)
	m.decodeFailTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xqlb_decode_fail_total",
			Help: "Total number of failed XQLB decode operations",
		},
		[]string{},
	)

	// MCP tools call metrics
	m.toolsCallTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_tools_call_total",
			Help: "Total number of MCP tools calls",
		},
		[]string{"tool", "result"},
	)
	m.toolsCallFail = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_tools_call_fail_total",
			Help: "Total number of failed MCP tools calls",
		},
		[]string{"tool"},
	)
	m.toolsCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_tools_call_duration_seconds",
			Help:    "Histogram of MCP tools call durations",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"tool"},
	)

	// Compile metrics
	m.compileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xqlb_compile_total",
			Help: "Total number of compile operations",
		},
		[]string{"target", "result"},
	)
	m.compileFail = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xqlb_compile_fail_total",
			Help: "Total number of failed compile operations",
		},
		[]string{"target"},
	)
	m.compileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "xqlb_compile_duration_seconds",
			Help:    "Histogram of compile durations",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"target"},
	)

	// Register all metrics with the custom registry
	registry.MustRegister(
		m.decodeTotal,
		m.decodeFailTotal,
		m.toolsCallTotal,
		m.toolsCallFail,
		m.toolsCallDuration,
		m.compileTotal,
		m.compileFail,
		m.compileDuration,
	)

	return m
}

// RecordDecodeSuccess increments successful decode counter.
func (m *MetricsCollector) RecordDecodeSuccess() {
	m.decodeTotal.WithLabelValues("success").Inc()
}

// RecordDecodeFail increments failed decode counter.
func (m *MetricsCollector) RecordDecodeFail() {
	m.decodeFailTotal.WithLabelValues().Inc()
}

// RecordToolsCall records MCP tools call duration and success/failure.
func (m *MetricsCollector) RecordToolsCall(tool string, duration float64, success bool) {
	result := "success"
	if !success {
		result = "failure"
		m.toolsCallFail.WithLabelValues(tool).Inc()
	}
	m.toolsCallTotal.WithLabelValues(tool, result).Inc()
	m.toolsCallDuration.WithLabelValues(tool).Observe(duration)
}

// RecordCompile records compile duration and success/failure.
func (m *MetricsCollector) RecordCompile(target string, duration float64, success bool) {
	result := "success"
	if !success {
		result = "failure"
		m.compileFail.WithLabelValues(target).Inc()
	}
	m.compileTotal.WithLabelValues(target, result).Inc()
	m.compileDuration.WithLabelValues(target).Observe(duration)
}

// PrometheusHandler returns an HTTP handler that exposes metrics in Prometheus format.
func (m *MetricsCollector) PrometheusHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Global metrics instance
var GlobalMetrics = NewMetricsCollector()