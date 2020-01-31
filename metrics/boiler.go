package metrics

import (
	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

var (
	boilerFlowTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: boiler,
		Name:      "flow_temperature_celsius",
		Help:      "Flow temperature as reported by the boiler.",
	})
	boilerReturnTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: boiler,
		Name:      "return_temperature_celsius",
		Help:      "Return temperature as reported by the boiler.",
	})
	outdoorTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "outdoor_temperature_celsius",
		Help:      "Temperature measured by the external sensor.",
	})
	tankTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: boiler,
		Name:      "tank_temperature_celsius",
		Help:      "Temperature of the water in the tank as reported by the boiler.",
	})
	electricalPowerConsumed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "electrical_power_consumption_watts_total",
		Help:      "The total amount of electrical power consumed.",
	}, []string{
		"time_frame",
		"magnitude",
		"component",
	})
)

func registerBoilerMetrics(r *prometheus.Registry) {
	r.MustRegister(boilerFlowTemperature)
	r.MustRegister(boilerReturnTemperature)
	r.MustRegister(outdoorTemperature)
	r.MustRegister(tankTemperature)
	r.MustRegister(electricalPowerConsumed)
}

func handleBoilerResponse(data []uint8) {
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
		log.Infof("return flow temp is %f", val.(float64))
	case 0x01D6:
		boilerFlowTemperature.Set(val.(float64))
		log.Infof("flow temp is %f", val.(float64))
	case 0x000C:
		outdoorTemperature.Set(val.(float64))
		log.Infof("outdoor temp is %f", val.(float64))
	case 0x000E:
		tankTemperature.Set(val.(float64))
		log.Infof("tank temp is %f", val.(float64))
	case 0x091A:
		log.Infof("El. Aufnahme WW Tag WH %f", val.(uint16))
	default:
		log.Debugf("boiler register %x unhandled", reg)
	}
}

func requestBoilerMetrics(bus *can.Bus) {
	// Temperaturen
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0016)) // Rücklauftemperatur
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x01D6)) // Vorlauftemperatur
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x000C)) // Aussentemperatur
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x000E)) // Speichertemperatur

	// Stromaufnahme
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091A)) // El. Aufnahme WW Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091B)) // El. Aufnahme WW Tag KWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091C)) // El. Aufnahme WW Sum KWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091D)) // El. Aufnahme WW Sum MWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091E)) // El. Aufnahme Heiz Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091F)) // El. Aufnahme Heiz Tag KWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0920)) // El. Aufnahme Heiz Sum KWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0921)) // El. Aufnahme Heiz Sum MWH

	// Waermeertrag
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092A)) // Waermeertrag WW Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092B)) // Waermeertrag WW Tag KWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092C)) // Waermeertrag WW Sum KWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092D)) // Waermeertrag WW Sum MWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092E)) // Waermeertrag Heiz Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092F)) // Waermeertrag Heiz Tag KWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0930)) // Waermeertrag Heiz Sum KWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0931)) // Waermeertrag Heiz Sum MWH
}
