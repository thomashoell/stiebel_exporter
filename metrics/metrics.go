package metrics

import (
	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
	"wlp_exporter/m/v2/stiebel"
)

const (
	namespace = "stiebel"
	heater    = "heater"
	boiler    = "boiler"
	manager   = "manager"

	day         = "day"
	total       = "total"
	wh          = "WH"
	kwh         = "kWH"
	mwh         = "MWH"
	water       = "water"
	heat        = "heat"
	we1         = "1"
	we2         = "2"
	compressor1 = "1"
	compressor2 = "2"
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
	registerMetrics()

	bus.SubscribeFunc(metricsHandler)

	log.Info("starting to gather metrics")
	go func() {
		for true {
			requestBoilerMetrics(bus)
			requestHeaterMetrics(bus)
			time.Sleep(10 * time.Second)
		}
	}()

	_ = bus.ConnectAndPublish()
}

func registerMetrics() {
	registry.MustRegister(boilerFlowTemperature)
	registry.MustRegister(boilerReturnTemperature)
	registry.MustRegister(outdoorTemperature)
	registry.MustRegister(tankTemperature)
	registry.MustRegister(electricalPowerConsumed)
	registry.MustRegister(heatProduced)
	registry.MustRegister(compressorSpeedRpm)
	registry.MustRegister(hotGasTemperature)
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
		handleHeaterResponse(data)
	case stiebel.Manager:
		handleManagerResponse(data)
	case stiebel.Kessel:
		handleBoilerResponse(data)
		//default:
		//	stiebel.LogFrame(frm)
	}
}
