package stiebel

import (
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
	"time"
)

// createReadFrame constructs a CAN bus request frame
func createReadFrame(sender uint16, receiver uint16, r *ElsterReading) *can.Frame {
	frm := can.Frame{
		ID:     uint32(sender),
		Length: 8,
		Data:   [8]uint8{},
	}
	copy(frm.Data[:], RequestFrame(receiver, r))
	return &frm
}

func ReadRegister(bus *can.Bus, sender uint16, receiver uint16, r *ElsterReading, ) {
	frm := createReadFrame(sender, receiver, r)

	for err := bus.Publish(*frm); err != nil; err = bus.Publish(*frm) {
		log.Error("error publishing frame on the CAN bus, backing off")
		time.Sleep(100 * time.Millisecond)
	}

	err := bus.Publish(*frm)

	if err != nil {
		log.Error(err)
	}
}
