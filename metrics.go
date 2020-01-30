package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type MetricsContainer struct {
	status prometheus.Gauge
}

const (
	namespace = "stiebel"
	subsystem = "backup"
)

var (
	registry = prometheus.NewRegistry()
	handler  = promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
)

func Handler() http.Handler {
	return handler
}

func GetMetrics() *MetricsContainer {
	metricsContainer := &MetricsContainer{status: prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "up",
		Help:        "meta metric",
		ConstLabels: map[string]string{"bla": "blub"},
	})}

	registry.MustRegister(metricsContainer.status)

	return metricsContainer
}
