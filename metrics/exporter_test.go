package metrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/expfmt"

	"github.com/thomashoell/stiebel_exporter/stiebel"
)

// fakeBus records published frames and fails the first failures calls.
type fakeBus struct {
	mu       sync.Mutex
	frames   []can.Frame
	failures int
	calls    int
}

func (b *fakeBus) Publish(frm can.Frame) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	if b.calls <= b.failures {
		return errors.New("bus busy")
	}
	b.frames = append(b.frames, frm)
	return nil
}

func newTestExporter(t *testing.T, bus stiebel.Publisher) (*Exporter, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	e, err := New(reg, bus, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	e.requestGap = 0
	e.publishBackoff = time.Millisecond
	return e, reg
}

// response builds a response frame of sender for index with the raw value v.
func response(sender uint16, index uint16, v uint16) can.Frame {
	data, err := stiebel.ResponseData(stiebel.PC, &stiebel.ElsterReading{Index: index, Type: stiebel.TypeNone}, float64(int16(v)))
	if err != nil {
		panic(err)
	}
	return can.Frame{ID: uint32(sender), Length: 8, Data: data}
}

func TestHandleFrame(t *testing.T) {
	tests := []struct {
		name   string
		sender uint16
		index  uint16
		value  uint16
		metric string
		want   string
	}{
		{"return temperature", stiebel.Kessel, 0x0016, 215, "stiebel_boiler_return_temperature_celsius", "stiebel_boiler_return_temperature_celsius 21.5"},
		{"flow temperature", stiebel.Kessel, 0x01D6, 300, "stiebel_boiler_flow_temperature_celsius", "stiebel_boiler_flow_temperature_celsius 30"},
		{"negative outdoor temperature", stiebel.Kessel, 0x000C, 0xFFF6, "stiebel_outdoor_temperature_celsius", "stiebel_outdoor_temperature_celsius -1"},
		{"tank temperature", stiebel.Kessel, 0x000E, 482, "stiebel_boiler_tank_temperature_celsius", "stiebel_boiler_tank_temperature_celsius 48.2"},
		{"electrical power water day Wh", stiebel.Kessel, 0x091A, 512, "stiebel_electrical_power_consumption_watt_hours",
			`stiebel_electrical_power_consumption_watt_hours{component="water",magnitude="WH",time_frame="day"} 512`},
		{"electrical power heat total MWh", stiebel.Kessel, 0x0921, 7, "stiebel_electrical_power_consumption_watt_hours",
			`stiebel_electrical_power_consumption_watt_hours{component="heat",magnitude="MWH",time_frame="total"} 7e+06`},
		{"heat WE1 water total kWh", stiebel.Kessel, 0x092C, 123, "stiebel_heat_produced_watt_hours",
			`stiebel_heat_produced_watt_hours{component="water",heat_generator="1",magnitude="kWH",time_frame="total"} 123000`},
		{"heat WE1 heat total MWh", stiebel.Kessel, 0x0931, 39, "stiebel_heat_produced_watt_hours",
			`stiebel_heat_produced_watt_hours{component="heat",heat_generator="1",magnitude="MWH",time_frame="total"} 3.9e+07`},
		{"heat WE2 from boiler", stiebel.Kessel, 0x0923, 2, "stiebel_heat_produced_watt_hours",
			`stiebel_heat_produced_watt_hours{component="water",heat_generator="2",magnitude="kWH",time_frame="day"} 2000`},
		{"heat WE2 from heater", stiebel.Heizmodul, 0x0926, 17, "stiebel_heat_produced_watt_hours",
			`stiebel_heat_produced_watt_hours{component="heat",heat_generator="2",magnitude="WH",time_frame="day"} 17`},
		{"compressor 1", stiebel.Heizmodul, 0x063E, 3600, "stiebel_compressor_speed_rpm", `stiebel_compressor_speed_rpm{unit="1"} 3600`},
		{"compressor 2", stiebel.Heizmodul, 0x063F, 0, "stiebel_compressor_speed_rpm", `stiebel_compressor_speed_rpm{unit="2"} 0`},
		{"hot gas temperature", stiebel.Heizmodul, 0x0265, 652, "stiebel_heater_hot_gas_temperature_celsius", "stiebel_heater_hot_gas_temperature_celsius 65.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, reg := newTestExporter(t, &fakeBus{})
			e.HandleFrame(response(tt.sender, tt.index, tt.value))

			got, err := testutil.CollectAndFormat(reg, expfmt.TypeTextPlain, tt.metric)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), tt.want+"\n") {
				t.Errorf("expected %q in\n%s", tt.want, got)
			}
		})
	}
}

func TestHandleFrameIgnored(t *testing.T) {
	tests := []struct {
		name string
		frm  can.Frame
		// responses counts responses of known units
		responses float64
	}{
		{"request", stiebel.RequestFrame(stiebel.PC, stiebel.Kessel, 0x000C), 0},
		{"short frame", can.Frame{ID: uint32(stiebel.Kessel), Length: 2, Data: [8]byte{0xD2, 0x00}}, 0},
		{"unknown sender", response(stiebel.Mischermodul, 0x000C, 100), 0},
		{"manager", response(stiebel.Manager, 0x0016, 100), 1},
		{"unknown index", response(stiebel.Kessel, 0xFFFF, 100), 1},
		{"index not exported", response(stiebel.Kessel, 0x0002, 100), 1},
		{"no value", response(stiebel.Kessel, 0x000C, 0x8000), 1},
		{"index exported by another unit", response(stiebel.Heizmodul, 0x000C, 100), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, reg := newTestExporter(t, &fakeBus{})
			e.HandleFrame(tt.frm)

			got, err := testutil.GatherAndCount(reg,
				"stiebel_outdoor_temperature_celsius",
				"stiebel_boiler_return_temperature_celsius")
			if err != nil {
				t.Fatal(err)
			}
			// gauges without labels always exist; their value must not have changed
			if v := testutil.ToFloat64(e.gauges.outdoorTemperature); v != 0 || got != 2 {
				t.Errorf("outdoor temperature changed to %v", v)
			}

			var responses float64
			for _, u := range []string{"boiler", "heater", "manager"} {
				responses += testutil.ToFloat64(e.responses.WithLabelValues(u))
			}
			if responses != tt.responses {
				t.Errorf("counted %v responses, want %v", responses, tt.responses)
			}
		})
	}
}

func TestResponsesAreCounted(t *testing.T) {
	e, _ := newTestExporter(t, &fakeBus{})
	before := time.Now().Unix()

	e.HandleFrame(response(stiebel.Kessel, 0x000C, 100))
	e.HandleFrame(response(stiebel.Kessel, 0x000E, 100))
	e.HandleFrame(response(stiebel.Heizmodul, 0x0265, 100))

	if v := testutil.ToFloat64(e.responses.WithLabelValues("boiler")); v != 2 {
		t.Errorf("boiler responses = %v, want 2", v)
	}
	if v := testutil.ToFloat64(e.responses.WithLabelValues("heater")); v != 1 {
		t.Errorf("heater responses = %v, want 1", v)
	}
	if v := testutil.ToFloat64(e.lastResponse.WithLabelValues("boiler")); v < float64(before) {
		t.Errorf("last response timestamp %v is before %v", v, before)
	}
}

func TestPoll(t *testing.T) {
	bus := &fakeBus{}
	e, _ := newTestExporter(t, bus)
	e.Poll(context.Background())

	type request struct{ receiver, index uint16 }
	var got []request
	for _, frm := range bus.frames {
		if frm.ID != uint32(stiebel.PC) {
			t.Errorf("request sent with ID %X, want %X", frm.ID, stiebel.PC)
		}
		msg, err := stiebel.ParseMessage(frm.Data[:])
		if err != nil {
			t.Fatal(err)
		}
		if msg.Type != stiebel.Read {
			t.Errorf("message type %X, want Read", msg.Type)
		}
		got = append(got, request{msg.Receiver, msg.Index})
	}

	// the same requests as before the refactoring, in the same order
	var want []request
	for _, i := range []uint16{0x0016, 0x01D6, 0x000C, 0x000E} {
		want = append(want, request{stiebel.Kessel, i})
	}
	for i := uint16(0x091A); i <= 0x0921; i++ {
		want = append(want, request{stiebel.Kessel, i})
	}
	for i := uint16(0x092A); i <= 0x0931; i++ {
		want = append(want, request{stiebel.Kessel, i})
	}
	for i := uint16(0x0922); i <= 0x0929; i++ {
		want = append(want, request{stiebel.Heizmodul, i})
	}
	for _, i := range []uint16{0x063E, 0x063F, 0x0265} {
		want = append(want, request{stiebel.Heizmodul, i})
	}

	if len(got) != len(want) {
		t.Fatalf("got %d requests, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestPollRetriesFailedPublish(t *testing.T) {
	bus := &fakeBus{failures: publishRetries}
	e, _ := newTestExporter(t, bus)
	e.units = e.units[:1]
	e.units[0].polled = e.units[0].polled[:1]

	e.Poll(context.Background())

	if len(bus.frames) != 1 {
		t.Errorf("expected the request to succeed on the last retry, got %d frames", len(bus.frames))
	}
	if v := testutil.ToFloat64(e.publishErrors); v != 0 {
		t.Errorf("publish errors = %v, want 0", v)
	}
}

func TestPollCountsPublishErrors(t *testing.T) {
	bus := &fakeBus{failures: 1000}
	e, _ := newTestExporter(t, bus)
	e.units = e.units[:1]
	e.units[0].polled = e.units[0].polled[:2]

	e.Poll(context.Background())

	if v := testutil.ToFloat64(e.publishErrors); v != 2 {
		t.Errorf("publish errors = %v, want 2", v)
	}
	if want := 2 * (publishRetries + 1); bus.calls != want {
		t.Errorf("publish called %d times, want %d", bus.calls, want)
	}
}

func TestRunStopsWhenCanceled(t *testing.T) {
	bus := &fakeBus{}
	e, _ := newTestExporter(t, bus)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.Run(ctx, time.Hour)
		close(done)
	}()

	// the first poll starts immediately
	deadline := time.After(5 * time.Second)
	for {
		bus.mu.Lock()
		n := len(bus.frames)
		bus.mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("no request was sent")
		case <-time.After(time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was canceled")
	}
}

func TestMetricNames(t *testing.T) {
	// the Grafana dashboards depend on these names
	_, reg := newTestExporter(t, &fakeBus{})

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, mf := range mfs {
		got[mf.GetName()] = true
	}
	for _, name := range []string{
		"stiebel_boiler_flow_temperature_celsius",
		"stiebel_boiler_return_temperature_celsius",
		"stiebel_outdoor_temperature_celsius",
		"stiebel_boiler_tank_temperature_celsius",
		"stiebel_heater_hot_gas_temperature_celsius",
		"stiebel_exporter_publish_errors_total",
	} {
		if !got[name] {
			t.Errorf("metric %s is missing", name)
		}
	}
}

func TestNewFailsOnDuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := New(reg, &fakeBus{}, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := New(reg, &fakeBus{}, logger); err == nil {
		t.Error("expected an error when registering the metrics twice")
	}
}
