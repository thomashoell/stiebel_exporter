package metrics

import (
	"encoding/binary"
	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

var (
	hotGasTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: heater,
		Name:      "hot_gas_temperature_celsius",
		Help:      "Hot gas temperature.",
	})
	condenserTemperature = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: heater,
		Name:      "condenser_temperature_celsius",
		Help:      "Condenser temperature.",
	})
)

func handleHeaterResponse(data []uint8) {
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
	case 0x0922:
		heatProduced.WithLabelValues(day, wh, water, we2).Set(float64(binary.BigEndian.Uint16(payload)))
	case 0x0923:
		heatProduced.WithLabelValues(day, kwh, water, we2).Set(float64(binary.BigEndian.Uint16(payload)) * 1e3)
	case 0x0924:
		heatProduced.WithLabelValues(total, kwh, water, we2).Set(float64(binary.BigEndian.Uint16(payload)) * 1e3)
	case 0x0925:
		heatProduced.WithLabelValues(total, mwh, water, we2).Set(float64(binary.BigEndian.Uint16(payload)) * 1e6)
	case 0x0926:
		heatProduced.WithLabelValues(day, wh, heat, we2).Set(float64(binary.BigEndian.Uint16(payload)))
	case 0x0927:
		heatProduced.WithLabelValues(day, kwh, heat, we2).Set(float64(binary.BigEndian.Uint16(payload)) * 1e3)
	case 0x0928:
		heatProduced.WithLabelValues(total, kwh, heat, we2).Set(float64(binary.BigEndian.Uint16(payload)) * 1e3)
	case 0x0929:
		heatProduced.WithLabelValues(total, mwh, heat, we2).Set(float64(binary.BigEndian.Uint16(payload)) * 1e6)
	case 0x063E:
		compressorSpeedRpm.WithLabelValues(compressor1).Set(float64(binary.BigEndian.Uint16(payload)))
	case 0x063F:
		compressorSpeedRpm.WithLabelValues(compressor2).Set(float64(binary.BigEndian.Uint16(payload)))
	case 0x0265:
		hotGasTemperature.Set(val.(float64))
	case 0x034f:
		condenserTemperature.Set(float64(binary.BigEndian.Uint16(payload)))
	default:
		log.Debugf("heater register %x unhandled", reg)
	}
}

func requestHeaterMetrics(bus *can.Bus) {
	// Waermeertrag WE2 (Zuheizer)
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0922)) // Waermeertrag WW Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0923)) // Waermeertrag WW Tag kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0924)) // Waermeertrag WW Sum kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0925)) // Waermeertrag WW Sum MWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0926)) // Waermeertrag Heiz Tag WH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0927)) // Waermeertrag Heiz Tag kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0928)) // Waermeertrag Heiz Sum kWH
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0929)) // Waermeertrag Heiz Sum MWH

	// Drehzahl Verdichter
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x063E)) // Verdichter 1
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x063F)) // Verdichter 2

	// Kühlmitteltemperaturen
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x0265)) // Heissgastemperatur
	stiebel.ReadRegister(bus, stiebel.PC, stiebel.Heizmodul, stiebel.Reading(0x034f)) // Verfluessigertemperatur

}
