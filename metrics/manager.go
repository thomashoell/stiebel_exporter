package metrics

import (
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

func HandleManagerResponse(data []uint8) {
	reg, payload := stiebel.Payload(data)
	val := stiebel.ValueString(payload)

	switch reg {
	case 0x0016:
		log.Infof("manager rücklauf ist %v", val)
	case 0x01D6:
		log.Infof("manager vorlauf ist %v", val)
	//default:
	//	log.Infof("manager register %x", reg)
	}
}
