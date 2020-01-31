package metrics

import (
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

func HandleBoilerResponse(data []uint8) {
	reg, payload := stiebel.Payload(data)
	val := stiebel.ValueString(payload)

	switch reg {
	case 0x0016:
		log.Infof("boiler rücklauf ist %v", val)
	case 0x01D6:
		log.Infof("boiler vorlauf ist %v", val)
		//default:
		//	log.Info("unknown register")
	}
}
