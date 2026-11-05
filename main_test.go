package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brutella/can"

	"github.com/thomashoell/wlp_exporter/stiebel"
)

func TestParseFlags(t *testing.T) {
	cfg, err := parseFlags(nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.device != "can0" || cfg.listen != ":9000" || cfg.interval != 10*time.Second || cfg.logLevel != slog.LevelInfo {
		t.Errorf("unexpected defaults: %+v", cfg)
	}

	cfg, err = parseFlags([]string{"-device", "can1", "-listen", "127.0.0.1:9100", "-interval", "30s", "-log-level", "debug"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.device != "can1" || cfg.listen != "127.0.0.1:9100" || cfg.interval != 30*time.Second || cfg.logLevel != slog.LevelDebug {
		t.Errorf("unexpected config: %+v", cfg)
	}

	for _, args := range [][]string{
		{"-interval", "0s"},
		{"-log-level", "verbose"},
		{"-unknown"},
		{"extra"},
	} {
		if _, err := parseFlags(args, io.Discard); err == nil {
			t.Errorf("parseFlags(%v): expected an error", args)
		}
	}

	if _, err := parseFlags([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp, got %v", err)
	}
}

// fakeInterface is a CAN interface for tests: written frames are recorded, frames passed to receive are read.
type fakeInterface struct {
	incoming chan []byte
	pending  []byte

	mu      sync.Mutex
	written bytes.Buffer
	closed  bool
}

func newFakeInterface() *fakeInterface {
	return &fakeInterface{incoming: make(chan []byte, 16)}
}

func (f *fakeInterface) receive(t *testing.T, frm can.Frame) {
	t.Helper()
	b, err := can.Marshal(frm)
	if err != nil {
		t.Fatal(err)
	}
	f.incoming <- b
}

func (f *fakeInterface) Read(b []byte) (int, error) {
	if len(f.pending) == 0 {
		data, ok := <-f.incoming
		if !ok {
			// like a closed socket; can.Bus would read again and again on io.EOF
			return 0, net.ErrClosed
		}
		f.pending = data
	}
	n := copy(b, f.pending)
	f.pending = f.pending[n:]
	return n, nil
}

func (f *fakeInterface) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.written.Write(b)
}

func (f *fakeInterface) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.closed = true
		close(f.incoming)
	}
	return nil
}

func (f *fakeInterface) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.written.Len() / 16 // frames are marshaled to 16 bytes
}

func TestServe(t *testing.T) {
	iface := newFakeInterface()
	bus := can.NewBus(can.NewReadWriteCloser(iface))

	ln, err := listen(t)
	if err != nil {
		t.Fatal(err)
	}
	url := "http://" + ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config{interval: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, logger, bus, ln) }()

	// the heat pump answers with an outdoor temperature of 12.3 °C
	data, err := stiebel.ResponseData(stiebel.PC, stiebel.Reading(0x000C), 12.3)
	if err != nil {
		t.Fatal(err)
	}
	iface.receive(t, can.Frame{ID: uint32(stiebel.Kessel), Length: 7, Data: data})

	body := waitFor(t, func() (string, bool) {
		body, err := get(url + "/metrics")
		return body, err == nil && strings.Contains(body, "stiebel_outdoor_temperature_celsius 12.3")
	})
	for _, want := range []string{"stiebel_exporter_build_info{", "go_goroutines", `stiebel_exporter_responses_total{unit="boiler"} 1`} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics does not contain %q", want)
		}
	}

	waitFor(t, func() (string, bool) { return "", iface.requests() > 0 })

	if body, err := get(url + "/"); err != nil || !strings.Contains(body, `href="/metrics"`) {
		t.Errorf("unexpected landing page: %q, %v", body, err)
	}
	if _, err := get(url + "/unknown"); err == nil {
		t.Error("expected 404 for an unknown path")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after the context was canceled")
	}
}

func TestServeFailsWhenBusCloses(t *testing.T) {
	iface := newFakeInterface()
	bus := can.NewBus(can.NewReadWriteCloser(iface))
	ln, err := listen(t)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- serve(context.Background(), config{interval: time.Hour}, slog.New(slog.NewTextHandler(io.Discard, nil)), bus, ln)
	}()

	_ = iface.Close()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "CAN bus closed") {
			t.Errorf("expected a CAN bus error, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after the bus was closed")
	}
}

func listen(t *testing.T) (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
}

func get(url string) (string, error) {
	resp, err := http.Get(url) //nolint:noctx // test against a local server
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return string(body), errors.New(resp.Status)
	}
	return string(body), nil
}

func waitFor(t *testing.T, cond func() (string, bool)) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		val, ok := cond()
		if ok {
			return val
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition not met in time, last value:\n%s", val)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
