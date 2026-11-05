package stiebel

import "testing"

func TestReading(t *testing.T) {
	tests := []struct {
		index uint16
		name  string
		typ   ElsterType
	}{
		{0x000C, "AUSSENTEMP", TypeDecVal},
		{0x01D6, "WPVORLAUFIST", TypeDecVal},
		{0x0931, "WAERMEERTRAG_HEIZ_SUM_MWH", TypeDoubleVal},
		{0x063E, "ISTDREHZAHL_VERDICHTER_1", TypeNone},
	}
	for _, tt := range tests {
		r := Reading(tt.index)
		if r == nil {
			t.Errorf("index %04X not found", tt.index)
			continue
		}
		if r.Index != tt.index || r.Name != tt.name || r.Type != tt.typ {
			t.Errorf("index %04X: got %+v", tt.index, *r)
		}
	}

	if r := Reading(0xFFFF); r != nil {
		t.Errorf("expected no reading for an unknown index, got %+v", *r)
	}
}

func TestReadingIndexComplete(t *testing.T) {
	for _, r := range readings {
		if Reading(r.Index) == nil {
			t.Errorf("index %04X (%s) is missing in the lookup table", r.Index, r.Name)
		}
	}
}

func TestErrorText(t *testing.T) {
	if text, ok := ErrorText(0x0004); !ok || text != "Hochdruck" {
		t.Errorf("got %q, %v", text, ok)
	}
	if _, ok := ErrorText(0xFFFF); ok {
		t.Error("expected no text for an unknown error")
	}
}

func TestOperatingModeName(t *testing.T) {
	if name, ok := OperatingModeName(0x0200); !ok || name != "Automatik" {
		t.Errorf("got %q, %v", name, ok)
	}
	if _, ok := OperatingModeName(0x0001); ok {
		t.Error("expected no name for an unknown mode")
	}
}
