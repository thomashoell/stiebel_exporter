package metrics

import (
	"encoding/binary"
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
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
}

