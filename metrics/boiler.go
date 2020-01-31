package metrics

import (
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
)

func HandleBoilerResponse(c can.Frame) {
	log.Info("Hi from boiler")
}
