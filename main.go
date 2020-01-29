package main

import (
	log "github.com/sirupsen/logrus"
	"wlp_exporter/make/stiebel"
)

func main() {

	log.SetLevel(log.DebugLevel)

	stiebel.DoSomething()
}
