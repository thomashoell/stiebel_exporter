package stiebel

import (
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
)

func DoSomething() {
	log.Info("starting to read CAN frames")
	bus, err := can.NewBusForInterfaceWithName("can0")

	if err != nil {
		log.Fatal("could not connect to bus: ", err)
	}

	//go func() {
	//	select {
	//	case <-time.After(5 * time.Second):
	//		log.Info("timeout expired, sending command")
	//		frm := can.Frame{
	//			ID:     0x680,
	//			Length: 5,
	//			Flags:  0,
	//			Res0:   0,
	//			Res1:   0,
	//			Data:   [8]uint8{0xA1, 0x00, 0xFA, 0x01, 0x9A},
	//		}
	//
	//		_ = bus.Publish(frm)
	//	}
	//}()

	bus.SubscribeFunc(logCANFrame)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)

	go func() {
		select {
		case sig := <-c:
			log.Infof("received signal %s, exiting", sig)
			_ = bus.Disconnect()
			os.Exit(1)
		}
	}()

	_ = bus.ConnectAndPublish()
}

func logCANFrame(frm can.Frame) {
	data := frm.Data
	module := data[0] >> 4
	action := data[0] & 0x0F
	target := data[1]<<8 + data[2]
	elsterIndex := int(data[3]<<8 + data[4])

	var measurement string
	var unit elsterUnit

	if target == 0x00fa {
		ei := elsterMap[elsterIndex]
		measurement = ei.Measurement
		unit = ei.Unit
	}
	if telegramType(action) == typeResponse {
		log.Infof("%-4x %-13s %-2s %-4s %-4x, %-4x", frm.ID, module, action, measurement, unit, data[5:6])
	}
}
