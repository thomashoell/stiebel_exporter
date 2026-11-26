package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/brutella/can"

	"github.com/thomashoell/stiebel_exporter/stiebel"
)

// Experimental write support: set the operating mode and read it back, once from the command line or from the web
// UI with -web-write (docs/roadmap.md, "Changing set points"). Notbetrieb is not offered – it heats with the
// electric heating elements only.

const programSwitch uint16 = 0x0112 // PROGRAMMSCHALTER

// writableOperatingModes are the operating modes -set-operating-mode accepts.
var writableOperatingModes = []string{
	"Bereitschaftsbetrieb", "Programmbetrieb", "Komfortbetrieb", "Eco-Betrieb", "Warmwasserbetrieb",
}

// writeBus is the part of *can.Bus used for a write.
type writeBus interface {
	stiebel.Publisher
	SubscribeFunc(fn can.HandlerFunc)
}

// operatingModeCode returns the value of an operating mode that may be written.
func operatingModeCode(name string) (uint16, error) {
	code, ok := stiebel.OperatingModeCode(name)
	if !ok || !slices.Contains(writableOperatingModes, name) {
		return 0, fmt.Errorf("operating mode %q cannot be set, allowed: %s", name,
			strings.Join(writableOperatingModes, ", "))
	}
	return code, nil
}

// modeWriter writes the operating mode to receiver and reads it back before and after. It subscribes to the bus
// once, so it can be shared by the web UI; writes run one at a time.
type modeWriter struct {
	bus      stiebel.Publisher
	receiver uint16
	settle   time.Duration
	timeout  time.Duration
	logger   *slog.Logger
	busy     chan struct{} // holds a token while a write runs
	values   chan uint16
}

func newModeWriter(bus writeBus, receiver uint16, settle, timeout time.Duration, logger *slog.Logger) *modeWriter {
	w := &modeWriter{
		bus:      bus,
		receiver: receiver,
		settle:   settle,
		timeout:  timeout,
		logger:   logger,
		busy:     make(chan struct{}, 1),
		values:   make(chan uint16, 4),
	}
	bus.SubscribeFunc(w.handleFrame)
	return w
}

// handleFrame passes on responses of the receiver for the program switch.
func (w *modeWriter) handleFrame(frm can.Frame) {
	length := min(int(frm.Length), len(frm.Data))
	msg, err := stiebel.ParseMessage(frm.Data[:length])
	if err != nil || msg.Type != stiebel.Response || frm.ID != uint32(w.receiver) || msg.Index != programSwitch ||
		len(msg.Value) != 2 {
		return
	}
	select {
	case w.values <- binary.BigEndian.Uint16(msg.Value):
	default:
	}
}

func (w *modeWriter) read(ctx context.Context) (uint16, error) {
	// drop values of earlier requests
	for len(w.values) > 0 {
		<-w.values
	}
	if err := w.bus.Publish(stiebel.RequestFrame(stiebel.PC, w.receiver, programSwitch)); err != nil {
		return 0, err
	}
	select {
	case v := <-w.values:
		return v, nil
	case <-time.After(w.timeout):
		return 0, errors.New("no response")
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// modeResult holds the operating modes read before and after a write; nil if not read.
type modeResult struct {
	before, after *uint16
}

// set writes the operating mode. Without a response to the first read it writes nothing.
func (w *modeWriter) set(ctx context.Context, name string) (modeResult, error) {
	var res modeResult
	code, err := operatingModeCode(name)
	if err != nil {
		return res, err
	}
	select {
	case w.busy <- struct{}{}:
		defer func() { <-w.busy }()
	case <-ctx.Done():
		return res, ctx.Err()
	}

	before, err := w.read(ctx)
	if err != nil {
		return res, fmt.Errorf("read the operating mode before writing: %w", err)
	}
	res.before = &before

	if err := w.bus.Publish(stiebel.WriteFrame(stiebel.PC, w.receiver, programSwitch, code)); err != nil {
		return res, fmt.Errorf("write: %w", err)
	}
	w.logger.Info("operating mode written", "mode", name, "before", describeMode(before),
		"receiver", fmt.Sprintf("0x%03X", w.receiver))

	select {
	case <-time.After(w.settle):
	case <-ctx.Done():
		return res, ctx.Err()
	}
	after, err := w.read(ctx)
	if err != nil {
		return res, fmt.Errorf("read the operating mode after writing: %w", err)
	}
	res.after = &after
	if after != code {
		return res, fmt.Errorf("the heat pump did not take the operating mode: %s instead of %s", describeMode(after),
			describeMode(code))
	}
	return res, nil
}

// OperatingModes returns the operating modes that may be set (web.ModeSetter).
func (w *modeWriter) OperatingModes() []string { return writableOperatingModes }

// SetOperatingMode sets the operating mode and returns the name read back (web.ModeSetter).
func (w *modeWriter) SetOperatingMode(ctx context.Context, name string) (string, error) {
	res, err := w.set(ctx, name)
	if err != nil {
		return "", err
	}
	after, _ := stiebel.OperatingModeName(*res.after)
	return after, nil
}

func describeMode(v uint16) string {
	n, _ := stiebel.OperatingModeName(v)
	return fmt.Sprintf("%s (0x%04X)", n, v)
}

// setOperatingMode writes the operating mode once and prints the frame and the modes before and after. With
// dryRun, it only prints the frame. The bus must be connected (receiving) unless dryRun is set.
func setOperatingMode(ctx context.Context, out io.Writer, logger *slog.Logger, bus writeBus, receiver uint16,
	name string, dryRun bool, settle, timeout time.Duration,
) error {
	code, err := operatingModeCode(name)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "write: %s\n", stiebel.FormatFrame(stiebel.WriteFrame(stiebel.PC, receiver, programSwitch, code)))
	if dryRun {
		return nil
	}
	res, err := newModeWriter(bus, receiver, settle, timeout, logger).set(ctx, name)
	if res.before != nil {
		_, _ = fmt.Fprintf(out, "before: %s\n", describeMode(*res.before))
	}
	if res.after != nil {
		_, _ = fmt.Fprintf(out, "after:  %s\n", describeMode(*res.after))
	}
	return err
}

// runSetOperatingMode opens the CAN interface and sets the operating mode once.
func runSetOperatingMode(ctx context.Context, cfg config, out io.Writer, logger *slog.Logger) error {
	if cfg.dryRun {
		return setOperatingMode(ctx, out, logger, nil, cfg.writeReceiver, cfg.setOperatingMode, true, 0, 0)
	}
	bus, err := can.NewBusForInterfaceWithName(cfg.device)
	if err != nil {
		return fmt.Errorf("cannot open CAN interface %s: %w", cfg.device, err)
	}
	go func() { _ = bus.ConnectAndPublish() }()
	defer func() { _ = bus.Disconnect() }()
	return setOperatingMode(ctx, out, logger, bus, cfg.writeReceiver, cfg.setOperatingMode, false,
		3*time.Second, 5*time.Second)
}
