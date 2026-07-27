//go:build !metrics

package server

import (
	"net/http"
)

// MetricsCollector is a zero-dependency no-op stub when built without -tags metrics.
type MetricsCollector struct{}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

func (m *MetricsCollector) RecordDecodeSuccess() {}

func (m *MetricsCollector) RecordDecodeFail() {}

func (m *MetricsCollector) RecordToolsCall(tool string, duration float64, success bool) {}

func (m *MetricsCollector) RecordCompile(target string, duration float64, success bool) {}

func (m *MetricsCollector) PrometheusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Prometheus metrics disabled. Recompile with -tags metrics to enable.\n"))
	})
}

var GlobalMetrics = NewMetricsCollector()
