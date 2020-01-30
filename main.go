package main

import (
	"context"
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
)

var device string

func main() {

	log.SetLevel(log.InfoLevel)

	device = "can0"

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
}
