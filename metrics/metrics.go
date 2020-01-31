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
	data := frm.Data[:]

	if data[0]&stiebel.Response == 0 {
		log.Debug("not a response")
		return
	}
	reg, payload := stiebel.Payload(data)

	switch reg {
	case 0x0016:
		log.Infof("rücklauf ist, %x, %x", payload, data[0])
	default:
		log.Infof("unhandled register %v", stiebel.Reading(reg))
	}
}
