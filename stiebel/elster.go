// Package stiebel implements the Elster protocol that Stiebel Eltron heat pumps speak on their CAN bus.
package stiebel

/*
   Elster frame decoding
   ---------------------
   https://www.haustechnikdialog.de/Forum/t/169910/ComfortSoft-mit-CAN-BUS-Anbindung-Raspberry-Pi-verwenden?page=7
   06 80  05  31 00  fa  09 31
   01 80  07  d2 00  fa  09 31  00 27
   06 80  05  31 00  fa  09 30
   01 80  07  d2 00  fa  09 30  00 73
   |---|  ||  |---|  ||  |---|  |---|
   1)     2)  3)     4)  5)     6)
   1) Sender CAN ID: 180 or 680
   2) No of bytes of data - 5 for queries, 7 for replies
   3) Receiver CAN ID of the communications partner and type of message
       Queries:   2nd digit is 1
                  Pattern:   n1 0m with n = 0x30, m = 0x00
                  Partner ID: 0x30 * 8 + 0x00 = 0x180
       Responses: 2nd digit is 2
                  Partner ID: 0xd0 * 8 + 0x00 = 0x680
   4) 0xFA indicates that the Elster index is greater than ff.
   5) Index (parameter) queried for: 0930 for kWh and 0931 for MWh
   6) Value returned 27h=39,73h=115
*/

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Message types (low nibble of the first data byte).
const (
	Write         byte = 0x00
	Read          byte = 0x01
	Response      byte = 0x02
	Ack           byte = 0x03
	WriteAck      byte = 0x04
	WriteRespond  byte = 0x05
	System        byte = 0x06
	SystemRespond byte = 0x07
)

// Broadcast is the receiver byte used for broadcast messages.
const Broadcast byte = 0x79

// CAN IDs of the bus participants,
// see https://github.com/Hunv/can2mqtt/blob/master/can2mqtt_core/can2mqtt_core/Translator/StiebelEltron/StiebelEltron.cs
const (
	Direkt          uint16 = 0x000
	Kessel          uint16 = 0x180
	Atez            uint16 = 0x280
	Bedienmodul     uint16 = 0x300 // possibly also 301, 302 and 303
	Raumfernfuehler uint16 = 0x400
	Manager         uint16 = 0x480
	Heizmodul       uint16 = 0x500
	Buskoppler      uint16 = 0x580
	Mischermodul    uint16 = 0x600 // possibly also 601, 602, 603 (wpwm controller)
	PC              uint16 = 0x680 // ComfortSoft, possibly also 6A1, 6A2
	Fremdgeraet     uint16 = 0x700
	DCFModul        uint16 = 0x780
)

// extendedIndex in the index byte announces a 16 bit index in the following two bytes.
const extendedIndex byte = 0xFA

var (
	// ErrNoValue is returned for the value 0x8000, which the heat pump sends if a value is not available.
	ErrNoValue = errors.New("no value available")
	// ErrShortMessage is returned for messages that are too short to contain an index.
	ErrShortMessage = errors.New("message too short")
)

// Message is a decoded Elster message.
type Message struct {
	// Type is the message type, e.g. Read or Response.
	Type byte
	// Receiver is the CAN ID of the participant the message is addressed to.
	Receiver uint16
	// Index is the Elster index (register) the message refers to.
	Index uint16
	// Value holds the two raw value bytes.
	Value []byte
}

// ParseMessage decodes the data bytes of a CAN frame. Value holds the (up to two) bytes following the index;
// requests have none.
func ParseMessage(data []byte) (Message, error) {
	if len(data) < 3 || data[2] == extendedIndex && len(data) < 5 {
		return Message{}, fmt.Errorf("%w: % X", ErrShortMessage, data)
	}

	msg := Message{
		Type:     data[0] & 0x0F,
		Receiver: ReceiverID(data),
	}

	valueOffset := 3
	if data[2] == extendedIndex {
		msg.Index = binary.BigEndian.Uint16(data[3:5])
		valueOffset = 5
	} else {
		msg.Index = uint16(data[2])
	}
	msg.Value = data[valueOffset:min(valueOffset+2, len(data))]

	return msg, nil
}

// ReceiverID returns the CAN ID of the receiver encoded in the first two data bytes.
func ReceiverID(b []byte) uint16 {
	return uint16(b[0]&0xF0)<<3 + uint16(b[1]&0x1F)
}

// EncodeReceiver writes the receiver CAN ID and the message type into the first two bytes of b.
func EncodeReceiver(b []byte, receiver uint16, messageType byte) {
	b[0] = byte(receiver>>3&0xF0) | messageType&0x0F
	b[1] = byte(receiver & 0x1F)
}

// EncodeIndex writes the Elster index into b and returns the offset of the value bytes.
func EncodeIndex(b []byte, index uint16) int {
	if index > 0xFF {
		b[2] = extendedIndex
		binary.BigEndian.PutUint16(b[3:], index)
		return 5
	}

	b[2] = byte(index)
	return 3
}

// RequestData returns the data bytes of a request for the value of an Elster index.
func RequestData(receiver uint16, index uint16) [8]byte {
	var b [8]byte
	EncodeReceiver(b[:], receiver, Read)
	EncodeIndex(b[:], index)
	return b
}

// ResponseData returns the data bytes of a message carrying a value for the given reading.
func ResponseData(receiver uint16, r *ElsterReading, val any) ([8]byte, error) {
	var b [8]byte
	encoded, err := EncodeValue(val, r.Type)
	if err != nil {
		return b, err
	}

	EncodeReceiver(b[:], receiver, Response)
	offset := EncodeIndex(b[:], r.Index)
	copy(b[offset:], encoded)
	return b, nil
}

// DecodeFloat decodes a numeric value. It fails for types that do not represent numbers.
func DecodeFloat(b []byte, t ElsterType) (float64, error) {
	val, err := DecodeValue(b, t)
	if err != nil {
		return 0, err
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case byte:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("value of type %T is not numeric", val)
	}
}

// DecodeValue decodes the two value bytes of a message. Depending on the type, the result is a
// float64, a byte, a bool or a string.
func DecodeValue(b []byte, t ElsterType) (any, error) {
	if len(b) != 2 {
		return nil, fmt.Errorf("value must be 2 bytes, got %d", len(b))
	}
	if bytes.Equal(b, []byte{0x80, 0x00}) {
		return nil, ErrNoValue
	}

	bigEndian := binary.BigEndian.Uint16(b)

	switch t {
	case TypeLittleEndian:
		return float64(signed(binary.LittleEndian.Uint16(b))), nil
	case TypeDecVal:
		return float64(signed(bigEndian)) / 10, nil
	case TypeCentVal:
		return float64(signed(bigEndian)) / 100, nil
	case TypeMilVal:
		return float64(signed(bigEndian)) / 1000, nil
	case TypeByte:
		return b[0], nil
	case TypeTime:
		return fmt.Sprintf("%02d:%02d", b[1], b[0]), nil
	case TypeDate:
		return fmt.Sprintf("%02d.%02d", b[0], b[1]), nil
	case TypeTimeDomain:
		if bigEndian&0x8080 != 0 {
			return nil, ErrNoValue
		}
		return fmt.Sprintf("%02d:%02d-%02d:%02d", b[0]/4, 15*(b[0]%4), b[1]/4, 15*(b[1]%4)), nil
	case TypeLittleBool:
		return decodeBool(b, []byte{0x01, 0x00})
	case TypeBool:
		return decodeBool(b, []byte{0x00, 0x01})
	default:
		return float64(signed(bigEndian)), nil
	}
}

func decodeBool(b []byte, trueValue []byte) (bool, error) {
	switch {
	case bytes.Equal(b, trueValue):
		return true, nil
	case bytes.Equal(b, []byte{0x00, 0x00}):
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool value % X", b)
	}
}

// EncodeValue encodes a value as the two value bytes of a message. Numeric types expect a float64,
// TypeByte a byte, the bool types a bool.
func EncodeValue(val any, t ElsterType) ([]byte, error) {
	b := []byte{0, 0}

	switch t {
	case TypeLittleEndian, TypeDecVal, TypeCentVal, TypeMilVal, TypeNone, TypeDoubleVal, TypeTripleVal:
		f, ok := val.(float64)
		if !ok {
			return nil, fmt.Errorf("expected float64, got %T", val)
		}
		scaled := math.Round(f * scale(t))
		if scaled < math.MinInt16 || scaled > math.MaxInt16 {
			return nil, fmt.Errorf("value %v out of range for type %d", f, t)
		}
		u := uint16(int16(scaled)) //nolint:gosec // two's complement of a value within the int16 range
		if t == TypeLittleEndian {
			binary.LittleEndian.PutUint16(b, u)
		} else {
			binary.BigEndian.PutUint16(b, u)
		}
	case TypeByte:
		v, ok := val.(byte)
		if !ok {
			return nil, fmt.Errorf("expected byte, got %T", val)
		}
		b[0] = v
	case TypeLittleBool, TypeBool:
		v, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", val)
		}
		if v && t == TypeLittleBool {
			b[0] = 0x01
		} else if v {
			b[1] = 0x01
		}
	default:
		return nil, fmt.Errorf("encoding of type %d is not supported", t)
	}

	return b, nil
}

func scale(t ElsterType) float64 {
	switch t {
	case TypeDecVal:
		return 10
	case TypeCentVal:
		return 100
	case TypeMilVal:
		return 1000
	default:
		return 1
	}
}

// signed interprets a 16 bit register value as two's complement.
func signed(u uint16) int16 {
	return int16(u) //nolint:gosec // reinterpretation is intended
}
