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

	bus.SubscribeFunc(handleCANFrame)

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

func handleCANFrame(frm can.Frame) {
	data := frm.Data
	module := data[0] >> 4
	if module != 0xd {

		return
	}

	action := data[0] & 0x0F
	target := uint16(data[1])<<8 + uint16(data[2])
	elsterIndex := uint16(data[3])<<8 + uint16(data[4])
	payload := uint16(data[5])<<8 + uint16(data[6])
	var measurement string
	var unit elsterUnit
	//
	if target == 0x00fa {
		ei := elsterMap[elsterIndex]
		measurement = ei.Measurement
		unit = ei.Unit
	}
	//if telegramType(action) == typeResponse {
	//	log.Infof("%-16x %-4x %-4x %-4x %-4s %-4x, %-4x", data, frm.ID, module, action, measurement, unit, data[5:6])
	//}
	log.Infof("%x %x %x %x %x %s %x\n", module, action, target, elsterIndex, payload, measurement, unit)
}
