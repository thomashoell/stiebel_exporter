package main

import (
	"context"
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

func main() {

	log.SetLevel(log.InfoLevel)

	device = "can0"
	sender = 0x680
	receiver = 0x500
	command = scan

	bus, err := can.NewBusForInterfaceWithName(device)
	if err != nil {
		log.Fatal(err)
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	signal.Notify(quit, os.Kill)

	srv := StartServer()

	go func() {
		select {
		case <-quit:
			_ = bus.Disconnect()

			ctx := context.Background()
			if err := srv.Shutdown(ctx); err != nil {
				log.Error("failed to shut down server properly", err)
			} else {
				log.Info("server was shut down properly")
			}

			os.Exit(1)
		}
	}()

	GatherMetrics(bus)

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
