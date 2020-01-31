package metrics

import (
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

func HandleBoilerResponse(data []uint8) {
	reg, payload := stiebel.Payload(data)

	switch reg {
	case 0x0016:
		log.Infof("boiler rücklauf ist %v", payload)
	case 0x01D6:
		log.Infof("boiler vorlauf ist %v", payload)
		//default:
		//	log.Info("unknown register")
	}
}
