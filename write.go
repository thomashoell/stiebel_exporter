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

// Experimental write support: set the operating mode once and read it back (docs/roadmap.md, "Changing set
// points"). Notbetrieb is not offered – it heats with the electric heating elements only.

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

// setOperatingMode writes the operating mode to receiver and reads it back before and after. With dryRun, it only
// prints the frame. The bus must be connected (receiving) unless dryRun is set.
func setOperatingMode(ctx context.Context, out io.Writer, logger *slog.Logger, bus writeBus, receiver uint16,
	name string, dryRun bool, settle, timeout time.Duration,
) error {
	code, err := operatingModeCode(name)
	if err != nil {
		return err
	}
	frame := stiebel.WriteFrame(stiebel.PC, receiver, programSwitch, code)
	_, _ = fmt.Fprintf(out, "write: %s\n", stiebel.FormatFrame(frame))
	if dryRun {
		return nil
	}

	values := make(chan uint16, 4)
	bus.SubscribeFunc(func(frm can.Frame) {
		length := min(int(frm.Length), len(frm.Data))
		msg, err := stiebel.ParseMessage(frm.Data[:length])
		if err != nil || msg.Type != stiebel.Response || frm.ID != uint32(receiver) || msg.Index != programSwitch ||
			len(msg.Value) != 2 {
			return
		}
		select {
		case values <- binary.BigEndian.Uint16(msg.Value):
		default:
		}
	})

	read := func() (uint16, error) {
		// drop values of earlier requests
		for len(values) > 0 {
			<-values
		}
		if err := bus.Publish(stiebel.RequestFrame(stiebel.PC, receiver, programSwitch)); err != nil {
			return 0, err
		}
		select {
		case v := <-values:
			return v, nil
		case <-time.After(timeout):
			return 0, errors.New("no response")
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	describe := func(v uint16) string {
		n, _ := stiebel.OperatingModeName(v)
		return fmt.Sprintf("%s (0x%04X)", n, v)
	}

	before, err := read()
	if err != nil {
		return fmt.Errorf("read the operating mode before writing: %w", err)
	}
	_, _ = fmt.Fprintf(out, "before: %s\n", describe(before))

	if err := bus.Publish(frame); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	logger.Info("operating mode written", "mode", name, "receiver", fmt.Sprintf("0x%03X", receiver))

	select {
	case <-time.After(settle):
	case <-ctx.Done():
		return ctx.Err()
	}
	after, err := read()
	if err != nil {
		return fmt.Errorf("read the operating mode after writing: %w", err)
	}
	_, _ = fmt.Fprintf(out, "after:  %s\n", describe(after))
	if after != code {
		return fmt.Errorf("the heat pump did not take the operating mode: %s instead of %s", describe(after),
			describe(code))
	}
	return nil
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
