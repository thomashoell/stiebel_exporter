package main

import (
	"context"
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"time"
	"wlp_exporter/m/v2/metrics"
)

var device string

func main() {

	configureLogging()

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

	metrics.GatherMetrics(bus)
}

func configureLogging() {
	log.SetLevel(log.InfoLevel)
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = time.RFC3339Nano
	log.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true
}
