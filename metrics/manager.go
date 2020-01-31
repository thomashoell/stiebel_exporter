package metrics

import (
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
)

func HandleManagerResponse(c can.Frame) {
	log.Info("Hi from manager")
}
