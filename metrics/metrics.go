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

	// not a response
	if data[0]&stiebel.Response == 0 {
		return
	}

	//rcvr := stiebel.ReceiverId(frm.Data[:2])
	sender := uint16(frm.ID)

	switch sender {
	case stiebel.Heizmodul:
		HandleHeaterResponse(data)
	case stiebel.Manager:
		HandleManagerResponse(data)
	case stiebel.Kessel:
		HandleBoilerResponse(data)
		//default:
		//	stiebel.LogFrame(frm)
	}
}
