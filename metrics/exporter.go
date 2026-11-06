// Package metrics polls the heat pump over the CAN bus and exports the responses as Prometheus metrics.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/thomashoell/stiebel_exporter/stiebel"
)

const (
	// publishRetries is the number of retries if a request can't be published on the bus.
	publishRetries = 3
	// publishBackoff is multiplied with the retry number to get the delay before a retry.
	publishBackoff = 100 * time.Millisecond
	// requestGap is the pause after each request, to not flood the bus.
	requestGap = 50 * time.Millisecond
)

// Exporter requests values from the heat pump and updates the metrics from the responses.
type Exporter struct {
	bus    stiebel.Publisher
	logger *slog.Logger
	gauges *gauges
	units  []*unit
	// requestGap and publishBackoff are fields so tests can shorten them
	requestGap     time.Duration
	publishBackoff time.Duration

	responses     *prometheus.CounterVec
	lastResponse  *prometheus.GaugeVec
	publishErrors prometheus.Counter
}

// New creates an Exporter that sends its requests to bus and registers its metrics with reg.
func New(reg prometheus.Registerer, bus stiebel.Publisher, logger *slog.Logger) (*Exporter, error) {
	g := newGauges()
	e := &Exporter{
		bus:            bus,
		logger:         logger,
		gauges:         g,
		units:          newUnits(g),
		requestGap:     requestGap,
		publishBackoff: publishBackoff,
		responses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "responses_total",
			Help:      "Responses received from the heat pump, by unit.",
		}, []string{"unit"}),
		lastResponse: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_response_timestamp_seconds",
			Help:      "Unix time of the last response received from the heat pump, by unit.",
		}, []string{"unit"}),
		publishErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "publish_errors_total",
			Help:      "Requests that could not be published on the CAN bus, even after retrying.",
		}),
	}

	collectors := append(g.collectors(), e.responses, e.lastResponse, e.publishErrors)
	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}

	return e, nil
}

// HandleFrame updates the metrics from a response frame. Other frames are ignored.
func (e *Exporter) HandleFrame(frm can.Frame) {
	length := min(int(frm.Length), len(frm.Data))
	msg, err := stiebel.ParseMessage(frm.Data[:length])
	if err != nil {
		e.logger.Debug("ignoring frame", "frame", stiebel.FormatFrame(frm), "error", err)
		return
	}
	if msg.Type != stiebel.Response {
		return
	}

	u := e.unit(uint16(frm.ID)) //nolint:gosec // CAN IDs of the heat pump are 11 bit
	if u == nil {
		e.logger.Debug("response from unknown sender", "frame", stiebel.FormatFrame(frm))
		return
	}

	e.responses.WithLabelValues(u.name).Inc()
	e.lastResponse.WithLabelValues(u.name).SetToCurrentTime()

	logger := e.logger.With("unit", u.name, "index", formatIndex(msg.Index))

	r := stiebel.Reading(msg.Index)
	if r == nil {
		logger.Debug("unknown index", "frame", stiebel.FormatFrame(frm))
		return
	}
	logger = logger.With("name", r.Name)

	setter, ok := u.setters[msg.Index]
	if !ok {
		logger.Debug("index not exported", "frame", stiebel.FormatFrame(frm))
		return
	}

	val, err := stiebel.DecodeFloat(msg.Value, r.Type)
	if errors.Is(err, stiebel.ErrNoValue) {
		logger.Debug("heat pump has no value")
		return
	}
	if err != nil {
		logger.Warn("cannot decode value", "frame", stiebel.FormatFrame(frm), "error", err)
		return
	}

	setter(val)
}

// Run requests the values of all polled indexes every interval until ctx is canceled.
func (e *Exporter) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		e.Poll(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Poll requests the values of all polled indexes once.
func (e *Exporter) Poll(ctx context.Context) {
	for _, u := range e.units {
		for _, index := range u.polled {
			if err := e.request(ctx, u.id, index); err != nil {
				if ctx.Err() != nil {
					return
				}
				e.publishErrors.Inc()
				e.logger.Error("cannot request value", "unit", u.name, "index", formatIndex(index), "error", err)
			}
			if !sleep(ctx, e.requestGap) {
				return
			}
		}
	}
}

// request publishes a request frame, retrying with an increasing delay if that fails.
func (e *Exporter) request(ctx context.Context, receiver uint16, index uint16) error {
	frm := stiebel.RequestFrame(stiebel.PC, receiver, index)

	err := e.bus.Publish(frm)
	for retry := 1; err != nil && retry <= publishRetries; retry++ {
		e.logger.Warn("cannot publish frame on the CAN bus, backing off", "retry", retry, "error", err)
		if !sleep(ctx, time.Duration(retry)*e.publishBackoff) {
			return ctx.Err()
		}
		err = e.bus.Publish(frm)
	}

	return err
}

func (e *Exporter) unit(id uint16) *unit {
	for _, u := range e.units {
		if u.id == id {
			return u
		}
	}
	return nil
}

// sleep waits for d and returns false if ctx was canceled before.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func formatIndex(index uint16) string {
	return fmt.Sprintf("0x%04X", index)
}
