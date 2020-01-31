package metrics

import (
	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

var (
	flowTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: heater,
		Name:      "flow_temperature_celsius",
		Help:      "Flow temperature in Celsius",
	})
	returnTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: heater,
		Name:      "return_temperature_celsius",
		Help:      "Return temperature in Celsius",
	})
)

func registerHeaterMetrics(r *prometheus.Registry) {
	r.MustRegister(flowTemperature)
	r.MustRegister(returnTemperature)
}

func HandleHeaterResponse(data []uint8) {
	reg, payload := stiebel.Payload(data)
	val := stiebel.ValueString(payload)

	switch reg {
	case 0x0016:
		log.Infof("heater rücklauf ist %v", val)
	case 0x01D6:
		log.Infof("heater vorlauf ist %v", val)
	//default:
	//	log.Infof("heater register %x", reg)
	}
}

func GatherHeaterMetrics(bus *can.Bus) {
	readMetricAndUpdateGauge(bus, flowTemperature, 0x01D6)
	readMetricAndUpdateGauge(bus, returnTemperature, 0x0016)
}

func readMetricAndUpdateGauge(bus *can.Bus, gauge prometheus.Gauge, register uint16) {
	val, err := stiebel.CanReadSynchronous(bus, stiebel.PC, stiebel.Heizmodul, register)
	if err != nil {
		log.Errorf("failed to read register %x", register)
	}

	switch v := val.(type) {
	case float64:
		gauge.Set(v)
	default:
		log.Errorf("unexpected type %v of returned value", v)
	}
}
