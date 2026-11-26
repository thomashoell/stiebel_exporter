package stiebel

import (
	"encoding/binary"

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

// WriteFrame returns a CAN frame in which sender sets an Elster index of receiver to the raw 16 bit value
// (big endian, like the heat pump sends it).
func WriteFrame(sender uint16, receiver uint16, index uint16, value uint16) can.Frame {
	var b [8]byte
	EncodeReceiver(b[:], receiver, Write)
	offset := EncodeIndex(b[:], index)
	binary.BigEndian.PutUint16(b[offset:], value)
	return can.Frame{
		ID:     uint32(sender),
		Length: uint8(offset + 2), //nolint:gosec // 5 or 7
		Data:   b,
	}
}
