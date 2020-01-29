package main

import (
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
)

type Command int

const (
	dump Command = iota
	scan
	read
	write
)

var command Command
var device string
var sender uint16
var receiver uint16
var register uint16
var value uint16
var numeric float64

func main() {

	log.SetLevel(log.DebugLevel)

	device = "can0"

	bus, err := can.NewBusForInterfaceWithName(device)
	if err != nil {
		log.Fatal(err)
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	signal.Notify(quit, os.Kill)

	go func() {
		select {
		case <-quit:
			bus.Disconnect()
			os.Exit(1)
		}
	}()

	switch command {
	case dump:
		CanDump(bus)
	case scan:
		CanScan(bus, sender, receiver)
	case read:
		CanRead(bus, sender, receiver, register)
	case write:
		CanWrite(bus, sender, receiver, register, value)
	}
}
