package bus

import (
	log "github.com/sirupsen/logrus"
	"net"
)

func GetCanInterface() {
	iFaces, err := net.Interfaces()

	if err != nil {
		log.Fatalf("failed to enumerate available interfaces: %s", err)
	}

	for _, iFace := range iFaces {
		log.Infof("found interface %s", iFace.Name)
	}
}
