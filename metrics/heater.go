package metrics

import (
	log "github.com/sirupsen/logrus"
	"wlp_exporter/stiebel"
)

var (

)



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

