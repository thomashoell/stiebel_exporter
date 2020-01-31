package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

var (
	boilerFlowTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: boiler,
		Name:      "flow_temperature_celsius",
		Help:      "Flow temperature in Celsius",
	})
	boilerReturnTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: boiler,
		Name:      "return_temperature_celsius",
		Help:      "Return temperature in Celsius",
	})
)

func registerBoilerMetrics(r *prometheus.Registry) {
	r.MustRegister(boilerFlowTemperature)
	r.MustRegister(boilerReturnTemperature)
}

func HandleBoilerResponse(data []uint8) {
	reg, payload := stiebel.Payload(data)
	r := stiebel.Reading(reg)

	if r == nil {
		log.Debugf("register %x is not handled", reg)
		return
	}

	val := stiebel.DecodeValue(payload, r.Type)

	if val == nil {
		log.Error("value is nil")
		return
	}

	switch reg {
	case 0x0016:
		boilerReturnTemperature.Set(val.(float64))
	case 0x01D6:
		boilerFlowTemperature.Set(val.(float64))
		//default:
		//	log.Infof("boiler register %x", reg)
	}
}
