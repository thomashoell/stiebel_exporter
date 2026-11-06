package stiebel

import (
	"github.com/brutella/can"
)

// Publisher sends CAN frames, e.g. a *can.Bus.
type Publisher interface {
	Publish(frame can.Frame) error
}

// RequestFrame returns a CAN frame in which sender asks receiver for the value of an Elster index.
func RequestFrame(sender uint16, receiver uint16, index uint16) can.Frame {
	return can.Frame{
		ID:     uint32(sender),
		Length: 8, // the unused bytes are zero
		Data:   RequestData(receiver, index),
	}
}
