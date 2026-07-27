//go:build metrics

package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector holds all Prometheus metrics for the server.
type MetricsCollector struct {
	decodeTotal       *prometheus.CounterVec
	decodeFailTotal   *prometheus.CounterVec
	toolsCallTotal    *prometheus.CounterVec
	toolsCallFail     *prometheus.CounterVec
	toolsCallDuration *prometheus.HistogramVec
	compileTotal      *prometheus.CounterVec
	compileFail       *prometheus.CounterVec
	compileDuration   *prometheus.HistogramVec
	registry          *prometheus.Registry
}

// NewMetricsCollector creates a new metrics collector with Prometheus.
func NewMetricsCollector() *MetricsCollector {
	registry := prometheus.NewRegistry()
	m := &MetricsCollector{registry: registry}

	m.decodeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "xqlb_decode_total", Help: "Total XQLB decodes"},
		[]string{"result"},
	)
	m.decodeFailTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "xqlb_decode_fail_total", Help: "Total failed XQLB decodes"},
		[]string{},
	)
	m.toolsCallTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "mcp_tools_call_total", Help: "Total MCP calls"},
		[]string{"tool", "result"},
	)
	m.toolsCallFail = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "mcp_tools_call_fail_total", Help: "Total failed MCP calls"},
		[]string{"tool"},
	)
	m.toolsCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_tools_call_duration_seconds",
			Help:    "Histogram of MCP call durations",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"tool"},
	)
	m.compileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "xqlb_compile_total", Help: "Total compile ops"},
		[]string{"target", "result"},
	)
	m.compileFail = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "xqlb_compile_fail_total", Help: "Total failed compiles"},
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

	registry.MustRegister(
		m.decodeTotal, m.decodeFailTotal,
		m.toolsCallTotal, m.toolsCallFail, m.toolsCallDuration,
		m.compileTotal, m.compileFail, m.compileDuration,
	)

	return m
}

func (m *MetricsCollector) RecordDecodeSuccess() {
	m.decodeTotal.WithLabelValues("success").Inc()
}

func (m *MetricsCollector) RecordDecodeFail() {
	m.decodeFailTotal.WithLabelValues().Inc()
}

func (m *MetricsCollector) RecordToolsCall(tool string, duration float64, success bool) {
	res := "success"
	if !success {
		res = "failure"
		m.toolsCallFail.WithLabelValues(tool).Inc()
	}
	m.toolsCallTotal.WithLabelValues(tool, res).Inc()
	m.toolsCallDuration.WithLabelValues(tool).Observe(duration)
}

func (m *MetricsCollector) RecordCompile(target string, duration float64, success bool) {
	res := "success"
	if !success {
		res = "failure"
		m.compileFail.WithLabelValues(target).Inc()
	}
	m.compileTotal.WithLabelValues(target, res).Inc()
	m.compileDuration.WithLabelValues(target).Observe(duration)
}

func (m *MetricsCollector) PrometheusHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

var GlobalMetrics = NewMetricsCollector()
