package stiebel

import (
	"errors"
	"testing"
)

// examples from https://www.haustechnikdialog.de/Forum/t/169910 (see elster.go)
var (
	exampleRequest  = []byte{0x31, 0x00, 0xFA, 0x09, 0x31}             // ComfortSoft (0x680) asks the boiler for 0x0931
	exampleResponse = []byte{0xD2, 0x00, 0xFA, 0x09, 0x31, 0x00, 0x27} // the boiler (0x180) answers 0x27
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want Message
	}{
		{
			name: "request with extended index",
			data: exampleRequest,
			want: Message{Type: Read, Receiver: Kessel, Index: 0x0931, Value: []byte{}},
		},
		{
			name: "response with extended index",
			data: exampleResponse,
			want: Message{Type: Response, Receiver: PC, Index: 0x0931, Value: []byte{0x00, 0x27}},
		},
		{
			name: "response with short index",
			data: []byte{0xD2, 0x00, 0x0C, 0x00, 0x7B, 0x00, 0x00, 0x00},
			want: Message{Type: Response, Receiver: PC, Index: 0x000C, Value: []byte{0x00, 0x7B}},
		},
		{
			name: "receiver with low bits",
			data: []byte{0xA2, 0x01, 0x16, 0xFF, 0xF6},
			want: Message{Type: Response, Receiver: Heizmodul + 1, Index: 0x0016, Value: []byte{0xFF, 0xF6}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMessage(tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Type != tt.want.Type || got.Receiver != tt.want.Receiver || got.Index != tt.want.Index ||
				string(got.Value) != string(tt.want.Value) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseMessageTooShort(t *testing.T) {
	for _, data := range [][]byte{nil, {0xD2, 0x00}, {0xD2, 0x00, 0xFA, 0x09}} {
		if _, err := ParseMessage(data); !errors.Is(err, ErrShortMessage) {
			t.Errorf("ParseMessage(% X): expected ErrShortMessage, got %v", data, err)
		}
	}
}

func TestReceiverRoundTrip(t *testing.T) {
	for _, id := range []uint16{Direkt, Kessel, Atez, Bedienmodul, Bedienmodul + 3, Manager, Heizmodul, PC, PC + 1, DCFModul} {
		for _, typ := range []byte{Write, Read, Response, SystemRespond} {
			b := make([]byte, 2)
			EncodeReceiver(b, id, typ)
			if got := ReceiverID(b); got != id {
				t.Errorf("receiver %X: decoded %X", id, got)
			}
			if got := b[0] & 0x0F; got != typ {
				t.Errorf("receiver %X: type %X decoded as %X", id, typ, got)
			}
		}
	}
}

func TestRequestData(t *testing.T) {
	tests := []struct {
		receiver, index uint16
		want            [8]byte
	}{
		{Kessel, 0x0931, [8]byte{0x31, 0x00, 0xFA, 0x09, 0x31}},
		{Kessel, 0x000C, [8]byte{0x31, 0x00, 0x0C}},
		{Heizmodul, 0x0265, [8]byte{0xA1, 0x00, 0xFA, 0x02, 0x65}},
	}
	for _, tt := range tests {
		if got := RequestData(tt.receiver, tt.index); got != tt.want {
			t.Errorf("RequestData(%X, %04X) = % X, want % X", tt.receiver, tt.index, got, tt.want)
		}
	}
}

func TestRequestFrame(t *testing.T) {
	frm := RequestFrame(PC, Kessel, 0x0931)
	if frm.ID != uint32(PC) || frm.Length != 8 || frm.Data != RequestData(Kessel, 0x0931) {
		t.Errorf("unexpected frame %+v", frm)
	}
}

func TestDecodeValue(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
		typ   ElsterType
		want  any
	}{
		{"positive dec", []byte{0x00, 0xFF}, TypeDecVal, 25.5},
		{"negative dec", []byte{0xFF, 0x01}, TypeDecVal, -25.5},
		{"cent", []byte{0x09, 0x29}, TypeCentVal, 23.45},
		{"mil", []byte{0xFC, 0x18}, TypeMilVal, -1.0},
		{"none is signed big endian", []byte{0x00, 0x27}, TypeNone, 39.0},
		{"double val", []byte{0x01, 0x2C}, TypeDoubleVal, 300.0},
		{"little endian", []byte{0x2C, 0x01}, TypeLittleEndian, 300.0},
		{"byte", []byte{0x2A, 0x00}, TypeByte, byte(0x2A)},
		{"time", []byte{0x1E, 0x0D}, TypeTime, "13:30"},
		{"date", []byte{0x18, 0x0C}, TypeDate, "24.12"},
		{"time domain", []byte{0x19, 0x5A}, TypeTimeDomain, "06:15-22:30"},
		{"bool true", []byte{0x00, 0x01}, TypeBool, true},
		{"bool false", []byte{0x00, 0x00}, TypeBool, false},
		{"little bool true", []byte{0x01, 0x00}, TypeLittleBool, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeValue(tt.value, tt.typ)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestDecodeValueErrors(t *testing.T) {
	tests := []struct {
		name    string
		value   []byte
		typ     ElsterType
		wantErr error
	}{
		{"no value", []byte{0x80, 0x00}, TypeDecVal, ErrNoValue},
		{"no time domain", []byte{0x80, 0x00 | 0x10}, TypeTimeDomain, ErrNoValue},
		{"invalid bool", []byte{0x01, 0x01}, TypeBool, nil},
		{"invalid little bool", []byte{0x00, 0x01}, TypeLittleBool, nil},
		{"short value", []byte{0x01}, TypeDecVal, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeValue(tt.value, tt.typ)
			if err == nil {
				t.Fatal("expected an error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestDecodeFloat(t *testing.T) {
	if got, err := DecodeFloat([]byte{0x00, 0x7B}, TypeDecVal); err != nil || got != 12.3 {
		t.Errorf("got %v, %v", got, err)
	}
	if got, err := DecodeFloat([]byte{0x07, 0x00}, TypeByte); err != nil || got != 7 {
		t.Errorf("got %v, %v", got, err)
	}
	if _, err := DecodeFloat([]byte{0x1E, 0x0D}, TypeTime); err == nil {
		t.Error("expected an error for a non-numeric type")
	}
	if _, err := DecodeFloat([]byte{0x80, 0x00}, TypeDecVal); !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got %v", err)
	}
}

func TestEncodeValueRoundTrip(t *testing.T) {
	tests := []struct {
		val any
		typ ElsterType
	}{
		{21.5, TypeDecVal},
		{-25.5, TypeDecVal},
		{23.45, TypeCentVal},
		{-1.234, TypeMilVal},
		{300.0, TypeLittleEndian},
		{-300.0, TypeNone},
		{byte(42), TypeByte},
		{true, TypeBool},
		{false, TypeBool},
		{true, TypeLittleBool},
	}

	for _, tt := range tests {
		b, err := EncodeValue(tt.val, tt.typ)
		if err != nil {
			t.Fatalf("EncodeValue(%v, %d): %v", tt.val, tt.typ, err)
		}
		got, err := DecodeValue(b, tt.typ)
		if err != nil {
			t.Fatalf("DecodeValue(% X, %d): %v", b, tt.typ, err)
		}
		if got != tt.val {
			t.Errorf("round trip of %v (type %d): got %v", tt.val, tt.typ, got)
		}
	}
}

func TestEncodeValueErrors(t *testing.T) {
	tests := []struct {
		val any
		typ ElsterType
	}{
		{"21.5", TypeDecVal},
		{4000.0, TypeDecVal}, // out of range
		{1, TypeByte},
		{1.0, TypeBool},
		{"13:30", TypeTime},
	}
	for _, tt := range tests {
		if _, err := EncodeValue(tt.val, tt.typ); err == nil {
			t.Errorf("EncodeValue(%v, %d): expected an error", tt.val, tt.typ)
		}
	}
}

func TestResponseData(t *testing.T) {
	b, err := ResponseData(PC, Reading(0x0931), 39.0)
	if err != nil {
		t.Fatal(err)
	}
	if want := [8]byte{0xD2, 0x00, 0xFA, 0x09, 0x31, 0x00, 0x27}; b != want {
		t.Errorf("got % X, want % X", b, want)
	}

	if _, err := ResponseData(PC, Reading(0x0931), "39"); err == nil {
		t.Error("expected an error for an invalid value")
	}
}
