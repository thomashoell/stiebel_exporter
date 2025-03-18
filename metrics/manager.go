package metrics

import (
	log "github.com/sirupsen/logrus"
	"wlp_exporter/m/v2/stiebel"
)

func handleManagerResponse(data []uint8) {
	reg, payload := stiebel.Payload(data)
	r := stiebel.Reading(reg)

	if r == nil {
		log.Debugf("register %x is not handled", reg)
		return
	}

	val := stiebel.DecodeValue(payload, r.Type)

	if val == nil {
		log.Errorf("[manager] response to register %x is nil. Complete frame is %x", reg, data)
		return
	}

	switch reg {
	case 0x0016:
		log.Debugf("manager rücklauf ist %v", val)
	case 0x01D6:
		log.Debugf("manager vorlauf ist %v", val)
	default:
		log.Debugf("manager register %x unhandled", reg)
	}
}
