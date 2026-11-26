package stiebel

import "testing"

func TestWriteFrame(t *testing.T) {
	// PC sets PROGRAMMSCHALTER (0x0112) of the boiler to Eco-Betrieb (0x0400)
	frm := WriteFrame(PC, Kessel, 0x0112, 0x0400)
	if frm.ID != uint32(PC) || frm.Length != 7 {
		t.Errorf("ID %X, length %d", frm.ID, frm.Length)
	}
	if want := [8]byte{0x30, 0x00, 0xFA, 0x01, 0x12, 0x04, 0x00}; frm.Data != want {
		t.Errorf("got % X, want % X", frm.Data, want)
	}

	msg, err := ParseMessage(frm.Data[:frm.Length])
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != Write || msg.Receiver != Kessel || msg.Index != 0x0112 {
		t.Errorf("parsed %+v", msg)
	}

	if short := WriteFrame(PC, Kessel, 0x0003, 480); short.Length != 5 || short.Data[2] != 0x03 {
		t.Errorf("short index: length %d, % X", short.Length, short.Data)
	}
}

func TestOperatingModeCode(t *testing.T) {
	for code, name := range operatingModes {
		if got, ok := OperatingModeCode(name); !ok || got != code {
			t.Errorf("%s: got %04X, %v", name, got, ok)
		}
	}
	if _, ok := OperatingModeCode("Tagbetrieb"); ok {
		t.Error("expected no code for an unknown name")
	}
}
