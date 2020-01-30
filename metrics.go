package main

import (
	"fmt"
	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type MetricsContainer struct {
	status prometheus.Gauge
}

const (
	namespace = "stiebel"
	sender    = 0x680
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
	metricsContainer := &MetricsContainer{status: prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "up",
		Help:        "meta metric",
		ConstLabels: map[string]string{"bla": "blub"},
	})}

	registry.MustRegister(metricsContainer.status)

	bus.SubscribeFunc(metricsHandler)
	log.Info("starting to gather metrics")
	_ = bus.ConnectAndPublish()
}

func gatherBasicInfo(bus *can.Bus) {

	const receiver = 0x500
}

func metricsHandler(frm can.Frame) {
	data := trimSuffix(frm.Data[:], 0x00)
	length := fmt.Sprintf("[%x]", frm.Length)

	chars := fmt.Sprintf("'%s'", printableString(data[:]))
	rcvr := ReceiverId(frm.Data[:2])

	if rcvr != sender {
		return
	}

	formatted := fmt.Sprintf("%-4x %-3s % -24X %-10s %6x ", frm.ID, length, data, chars, rcvr)

	reg, payload := Payload(data)
	formatted += fmt.Sprintf("%04X ", reg)

	if data[0]&Data != 0 {
		if r := Reading(reg); r != nil {
			val := DecodeValue(payload, r.Type)
			valStr := ValueString(val)

			formatted += fmt.Sprintf("%-24s %11s", left(r.Name, 20), valStr)
		}
	}

	log.Info(formatted)
}
