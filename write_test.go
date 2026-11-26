package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brutella/can"

	"github.com/thomashoell/stiebel_exporter/stiebel"
)

// fakeHeatPump answers requests for the operating mode and takes writes unless readOnly is set.
type fakeHeatPump struct {
	mu       sync.Mutex
	mode     uint16
	readOnly bool
	handler  can.HandlerFunc
	writes   int
}

func (f *fakeHeatPump) SubscribeFunc(fn can.HandlerFunc) { f.handler = fn }

func (f *fakeHeatPump) Publish(frm can.Frame) error {
	msg, err := stiebel.ParseMessage(frm.Data[:frm.Length])
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch msg.Type {
	case stiebel.Write:
		f.writes++
		if !f.readOnly {
			f.mode = binary.BigEndian.Uint16(msg.Value)
		}
	case stiebel.Read:
		data, err := stiebel.ResponseData(stiebel.PC, &stiebel.ElsterReading{Index: msg.Index, Type: stiebel.TypeNone},
			float64(int16(f.mode)))
		if err != nil {
			return err
		}
		go f.handler(can.Frame{ID: uint32(msg.Receiver), Length: 7, Data: data})
	}
	return nil
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestSetOperatingMode(t *testing.T) {
	hp := &fakeHeatPump{mode: 0x0300}
	var out bytes.Buffer
	err := setOperatingMode(context.Background(), &out, discard, hp, stiebel.Kessel, "Eco-Betrieb", false,
		time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"before: Komfortbetrieb (0x0300)", "after:  Eco-Betrieb (0x0400)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, out.String())
		}
	}
	if hp.writes != 1 {
		t.Errorf("%d writes, want 1", hp.writes)
	}
}

func TestSetOperatingModeNotTaken(t *testing.T) {
	hp := &fakeHeatPump{mode: 0x0300, readOnly: true}
	err := setOperatingMode(context.Background(), io.Discard, discard, hp, stiebel.Kessel, "Eco-Betrieb", false,
		time.Millisecond, time.Second)
	if err == nil || !strings.Contains(err.Error(), "did not take") {
		t.Errorf("expected an error, got %v", err)
	}
}

func TestSetOperatingModeDryRun(t *testing.T) {
	var out bytes.Buffer
	if err := setOperatingMode(context.Background(), &out, discard, nil, stiebel.Kessel, "Eco-Betrieb", true, 0, 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "write: ") {
		t.Errorf("unexpected output %q", out.String())
	}
}

func TestSetOperatingModeRejectsNotbetrieb(t *testing.T) {
	for _, name := range []string{"Notbetrieb", "Tagbetrieb", ""} {
		if _, err := operatingModeCode(name); err == nil {
			t.Errorf("%q: expected an error", name)
		}
	}
}

func TestSetOperatingModeNoResponse(t *testing.T) {
	hp := &silentBus{}
	err := setOperatingMode(context.Background(), io.Discard, discard, hp, stiebel.Kessel, "Eco-Betrieb", false,
		time.Millisecond, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "no response") {
		t.Errorf("expected no response, got %v", err)
	}
	if hp.published != 1 {
		t.Errorf("%d frames published, want only the first request (no write without knowing the current mode)", hp.published)
	}
}

type silentBus struct{ published int }

func (b *silentBus) SubscribeFunc(can.HandlerFunc) {}
func (b *silentBus) Publish(can.Frame) error       { b.published++; return nil }

func TestModeWriterSetOperatingMode(t *testing.T) {
	hp := &fakeHeatPump{mode: 0x0300}
	w := newModeWriter(hp, stiebel.Manager, time.Millisecond, time.Second, discard)
	for _, name := range []string{"Eco-Betrieb", "Komfortbetrieb"} {
		got, err := w.SetOperatingMode(context.Background(), name)
		if err != nil || got != name {
			t.Errorf("%s: got %q, %v", name, got, err)
		}
	}
	if hp.writes != 2 {
		t.Errorf("%d writes, want 2", hp.writes)
	}
	if _, err := w.SetOperatingMode(context.Background(), "Notbetrieb"); err == nil {
		t.Error("Notbetrieb: expected an error")
	}
}

func TestModeWriterWaitsForRunningWrite(t *testing.T) {
	w := newModeWriter(&silentBus{}, stiebel.Manager, time.Millisecond, time.Second, discard)
	w.busy <- struct{}{} // a write is running
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := w.SetOperatingMode(ctx, "Eco-Betrieb"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected the deadline to pass while waiting, got %v", err)
	}
}
