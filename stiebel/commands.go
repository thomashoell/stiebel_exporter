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

func ReadRegister(bus *can.Bus, sender uint16, receiver uint16, r *ElsterReading) {
	frm := createReadFrame(sender, receiver, r)

	// try to publish a frame on the bus. If that fails, back off at most 3 times for an increasing amount of time
	for err, retry := bus.Publish(*frm), 1; err != nil && retry <= 3; err, retry = bus.Publish(*frm), retry+1 {
		log.Error("error publishing frame on the CAN bus, backing off", retry)
		time.Sleep(time.Duration(100*retry) * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // give the bus a little time
}
