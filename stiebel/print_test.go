package stiebel

import (
	"strings"
	"testing"

	"github.com/brutella/can"
)

func TestFormatFrame(t *testing.T) {
	response := can.Frame{ID: uint32(Kessel), Length: 7, Data: [8]byte{0xD2, 0x00, 0xFA, 0x09, 0x31, 0x00, 0x27}}
	got := FormatFrame(response)
	for _, want := range []string{"180", "[7]", "D2 00 FA 09 31 00 27", " 680 0931", "WAERMEERTRAG_HEIZ_SUM_MWH", "39.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q does not contain %q", got, want)
		}
	}

	request := RequestFrame(PC, Kessel, 0x0931)
	got = FormatFrame(request)
	if strings.Contains(got, "WAERMEERTRAG") {
		t.Errorf("request should not be decoded: %q", got)
	}

	noValue := can.Frame{ID: uint32(Kessel), Length: 7, Data: [8]byte{0xD2, 0x00, 0xFA, 0x09, 0x31, 0x80, 0x00}}
	if got := FormatFrame(noValue); !strings.Contains(got, ErrNoValue.Error()) {
		t.Errorf("%q does not mention the missing value", got)
	}

	short := can.Frame{ID: uint32(Kessel), Length: 2, Data: [8]byte{0xD2, 0x00}}
	if got := FormatFrame(short); !strings.Contains(got, "D2 00") {
		t.Errorf("unexpected format of a short frame: %q", got)
	}
}

func TestValueString(t *testing.T) {
	tests := []struct {
		val  any
		want string
	}{
		{21.53, "21.5"},
		{"13:30", "13:30"},
		{true, "true"},
		{nil, "<nil>"},
		{byte(10), "0x000A"},
	}
	for _, tt := range tests {
		if got := ValueString(tt.val); got != tt.want {
			t.Errorf("ValueString(%v) = %q, want %q", tt.val, got, tt.want)
		}
	}
}

func TestPrintableString(t *testing.T) {
	if got := printableString([]byte{'A', 0x00, 'z', 0x7F}); got != "A.z." {
		t.Errorf("got %q", got)
	}
}
