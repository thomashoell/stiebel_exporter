package metrics

import (
	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
	"wlp_exporter/stiebel"
)

const (
	namespace = "stiebel"
	heater    = "heizmodul"
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

func GatherMetrics(bus *can.Bus) {
	registerHeaterMetrics(registry)
	bus.SubscribeFunc(metricsHandler)
	log.Info("starting to gather metrics")
	go func() {
		for true {
			//GatherHeaterMetrics(bus)
			time.Sleep(10 * time.Second)
		}
	}()

	_ = bus.ConnectAndPublish()
}

func metricsHandler(frm can.Frame) {
	rcvr := stiebel.ReceiverId(frm.Data[:2])

	switch rcvr {
	case stiebel.Heizmodul:
		HandleHeaterResponse(frm)
	case stiebel.Manager:
		HandleManagerResponse(frm)
	case stiebel.Kessel:
		HandleBoilerResponse(frm)
	default:
		stiebel.LogFrame(frm)
	}
}
