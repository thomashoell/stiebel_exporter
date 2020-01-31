package stiebel

import (
	"errors"
	"github.com/brutella/can"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	ReadRegisterTimeoutMillies = 250
)

func CanDump(bus *can.Bus) {
	bus.SubscribeFunc(LogFrame)
	bus.ConnectAndPublish()
}

// makeScanMatcher creates a function that matches send/recv/reg for incoming frames
func makeScanMatcher(
	c chan can.Frame,
	sender uint16,
	receiver uint16,
	register uint16,
) func(frm can.Frame) {
	return func(frm can.Frame) {
		frmReceiver := ReceiverId(frm.Data[:2])

		// frame sent back from receiver to sender?
		if uint16(frm.ID) == receiver && frmReceiver == sender {
			// requested register?
			if reg, _ := Payload(frm.Data[:]); reg == register {
				// data frame?
				if frm.Data[0]&Response != 0 {
					c <- frm
				}
			}
		}
	}
}

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

func readRegister(
	bus *can.Bus,
	sender uint16,
	receiver uint16,
	r *ElsterReading,
) *can.Frame {
	c := make(chan can.Frame) // signalling channel
	frm := createReadFrame(sender, receiver, r)

	handler := can.NewHandler(makeScanMatcher(c, sender, receiver, r.Index))
	bus.Subscribe(handler)
	defer bus.Unsubscribe(handler)

	startTime := time.Now()
	_ = bus.Publish(*frm)
	select {
	case <-time.After(ReadRegisterTimeoutMillies * time.Millisecond):
		// timeout
		return nil
	case frm := <-c:
		// result
		duration := time.Since(startTime)
		log.Debugf("CAN read took: %.fms", duration.Seconds()*1e3)
		return &frm
	}
}
// reads a register in a blocking way
func CanReadSynchronous(bus *can.Bus, sender uint16, receiver uint16, register uint16) (interface{}, error) {
	r := Reading(register)
	if r == nil {
		log.Errorf("Unknown register '%X'", register)
		return nil, errors.New("failed to read register")
	}

	go bus.ConnectAndPublish()
	defer bus.Disconnect()

	frm := readRegister(bus, sender, receiver, r)
	if frm == nil {
		log.Error("failed to get response within specified timeout")
		return nil, errors.New("failed to read register")
	}

	_, payload := Payload(frm.Data[:])
	val := DecodeValue(payload, r.Type)
	return val, nil
}
