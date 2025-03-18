package metrics

import (
	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"wlp_exporter/m/v2/stiebel"
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
		Name:      "electrical_power_consumption_watt_hours",
		Help:      "The total amount of electrical power consumed.",
	}, []string{
		"time_frame",
		"magnitude",
		"component",
	})
	heatProduced = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "heat_produced_watt_hours",
		Help:      "The total amount of heat generated.",
	}, []string{
		"time_frame",
		"magnitude",
		"component",
		"heat_generator",
	})
	compressorSpeedRpm = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "compressor_speed_rpm",
		Help:      "The compressor's revolutions per minute.",
	}, []string{
		"unit",
	})
)

func handleBoilerResponse(data []uint8) {
	reg, payload := stiebel.Payload(data)
	r := stiebel.Reading(reg)

	if r == nil {
		log.Debugf("register %x is not handled", reg)
		return
	}

	val := stiebel.DecodeValue(payload, r.Type)

	if val == nil {
		log.Error("[boiler] response to register %x is nil. Complete frame is %x", reg, data)
		return
	}

	switch reg {
	case 0x0016:
		boilerReturnTemperature.Set(val.(float64))
	case 0x01D6:
		boilerFlowTemperature.Set(val.(float64))
	case 0x000C:
		outdoorTemperature.Set(val.(float64))
	case 0x000E:
		tankTemperature.Set(val.(float64))
	case 0x091A:
		electricalPowerConsumed.WithLabelValues(day, wh, water).Set(val.(float64))
	case 0x091B:
		electricalPowerConsumed.WithLabelValues(day, kwh, water).Set(val.(float64) * 1e3)
	case 0x091C:
		electricalPowerConsumed.WithLabelValues(total, kwh, water).Set(val.(float64) * 1e3)
	case 0x091D:
		electricalPowerConsumed.WithLabelValues(total, mwh, water).Set(val.(float64) * 1e6)
	case 0x091E:
		electricalPowerConsumed.WithLabelValues(day, wh, heat).Set(val.(float64))
	case 0x091F:
		electricalPowerConsumed.WithLabelValues(day, kwh, heat).Set(val.(float64) * 1e3)
	case 0x0920:
		electricalPowerConsumed.WithLabelValues(total, kwh, heat).Set(val.(float64) * 1e3)
	case 0x0921:
		electricalPowerConsumed.WithLabelValues(total, mwh, heat).Set(val.(float64) * 1e6)
	case 0x0922:
		heatProduced.WithLabelValues(day, wh, water, we2).Set(val.(float64))
	case 0x0923:
		heatProduced.WithLabelValues(day, kwh, water, we2).Set(val.(float64) * 1e3)
	case 0x0924:
		heatProduced.WithLabelValues(total, kwh, water, we2).Set(val.(float64) * 1e3)
	case 0x0925:
		heatProduced.WithLabelValues(total, mwh, water, we2).Set(val.(float64) * 1e6)
	case 0x0926:
		heatProduced.WithLabelValues(day, wh, heat, we2).Set(val.(float64))
	case 0x0927:
		heatProduced.WithLabelValues(day, kwh, heat, we2).Set(val.(float64) * 1e3)
	case 0x0928:
		heatProduced.WithLabelValues(total, kwh, heat, we2).Set(val.(float64) * 1e3)
	case 0x0929:
		heatProduced.WithLabelValues(total, mwh, heat, we2).Set(val.(float64) * 1e6)
	case 0x092A:
		heatProduced.WithLabelValues(day, wh, water, we1).Set(val.(float64))
	case 0x092B:
		heatProduced.WithLabelValues(day, kwh, water, we1).Set(val.(float64) * 1e3)
	case 0x092C:
		heatProduced.WithLabelValues(total, kwh, water, we1).Set(val.(float64) * 1e3)
	case 0x092D:
		heatProduced.WithLabelValues(total, mwh, water, we1).Set(val.(float64) * 1e6)
	case 0x092E:
		heatProduced.WithLabelValues(day, wh, heat, we1).Set(val.(float64))
	case 0x092F:
		heatProduced.WithLabelValues(day, kwh, heat, we1).Set(val.(float64) * 1e3)
	case 0x0930:
		heatProduced.WithLabelValues(total, kwh, heat, we1).Set(val.(float64) * 1e3)
	case 0x0931:
		heatProduced.WithLabelValues(total, mwh, heat, we1).Set(val.(float64) * 1e6)
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
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091B)) // El. Aufnahme WW Tag kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091C)) // El. Aufnahme WW Sum kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091D)) // El. Aufnahme WW Sum MWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091E)) // El. Aufnahme Heiz Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x091F)) // El. Aufnahme Heiz Tag kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0920)) // El. Aufnahme Heiz Sum kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0921)) // El. Aufnahme Heiz Sum MWH

	// Waermeertrag WE1
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092A)) // Waermeertrag WW Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092B)) // Waermeertrag WW Tag kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092C)) // Waermeertrag WW Sum kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092D)) // Waermeertrag WW Sum MWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092E)) // Waermeertrag Heiz Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x092F)) // Waermeertrag Heiz Tag kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0930)) // Waermeertrag Heiz Sum kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Kessel, stiebel.Reading(0x0931)) // Waermeertrag Heiz Sum MWH
}
