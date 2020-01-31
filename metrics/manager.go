package metrics

import (
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

func HandleManagerResponse(data []uint8) {
	reg, payload := stiebel.Payload(data)

	switch reg {
	case 0x0016:
		log.Infof("manager rücklauf ist %v", payload)
	case 0x01D6:
		log.Infof("manager vorlauf ist %v", payload)
		//default:
		//	log.Info("unknown register")
	}
}
