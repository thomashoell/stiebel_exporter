package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var now = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

// testRegistry holds a few metrics like the exporter registers them.
func testRegistry(t *testing.T, answered bool, mode string) *prometheus.Registry {
	t.Helper()
	reg := prometheus.NewRegistry()
	gauge := func(name string, v float64) {
		g := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: name})
		g.Set(v)
		reg.MustRegister(g)
	}
	gauge("stiebel_outdoor_temperature_celsius", -3.27)
	gauge("stiebel_boiler_tank_temperature_celsius", 46.8)
	gauge("stiebel_boiler_tank_comfort_setpoint_celsius", 48)
	gauge("stiebel_boiler_tank_eco_setpoint_celsius", 47)
	gauge("stiebel_compressor_running", 1)
	gauge("stiebel_heater_defrost_active", 0)
	gauge("stiebel_compressor_modulation_percent", 43)
	gauge("stiebel_heater_last_defrost_duration_seconds", 192)

	modes := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stiebel_operating_mode", Help: "mode"}, []string{"mode"})
	modes.WithLabelValues(mode).Set(768)
	reg.MustRegister(modes)

	el := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: electrical, Help: "el"},
		[]string{"time_frame", "magnitude", "component"})
	el.WithLabelValues("total", "kWH", "heat").Set(297e3)
	el.WithLabelValues("total", "MWH", "heat").Set(17e6)
	el.WithLabelValues("total", "kWH", "water").Set(170e3)
	el.WithLabelValues("total", "MWH", "water").Set(5e6)
	reg.MustRegister(el)

	if answered {
		last := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "stiebel_exporter_last_response_timestamp_seconds", Help: "last",
		}, []string{"unit"})
		last.WithLabelValues("boiler").Set(float64(now.Add(-3 * time.Second).Unix()))
		last.WithLabelValues("heater").Set(float64(now.Add(-2 * time.Minute).Unix()))
		reg.MustRegister(last)
	}
	return reg
}

func testServer(t *testing.T, reg *prometheus.Registry) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux, reg, Info{Version: "2.4.0", Commit: "abc1234"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(body)
}

func TestValues(t *testing.T) {
	s, err := collect(testRegistry(t, true, "Komfortbetrieb"), now)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"outdoor":          "-3,3 °C",
		"tank":             "46,8 °C",
		"tank_setpoint":    "48,0 °C",
		"compressor":       "läuft",
		"modulation":       "43 %",
		"last_defrost":     "3:12 min",
		"electrical_total": "22,47 MWh",
		"age_boiler":       "vor 3 s",
		"age_heater":       "vor 2 min",
	}
	got := map[string]value{}
	for _, sec := range sections {
		for _, it := range sec.items {
			got[it.key] = it.evaluate(s)
		}
	}
	for key, text := range want {
		if got[key].Text != text {
			t.Errorf("%s: got %q, want %q", key, got[key].Text, text)
		}
	}
	if got["age_heater"].State != "warn" || got["age_boiler"].State != "on" {
		t.Errorf("age states: boiler %q, heater %q", got["age_boiler"].State, got["age_heater"].State)
	}
	if got["compressor"].State != "on" {
		t.Errorf("compressor state %q", got["compressor"].State)
	}
}

func TestEcoSetpoint(t *testing.T) {
	s, err := collect(testRegistry(t, true, "Eco-Betrieb"), now)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := s.tankSetpoint(); !ok || v != 47 {
		t.Errorf("effective set point in Eco-Betrieb: %v, %v", v, ok)
	}
}

func TestNoDataBeforeFirstResponse(t *testing.T) {
	s, err := collect(testRegistry(t, false, "Komfortbetrieb"), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range sections {
		for _, it := range sec.items {
			if v := it.evaluate(s); v.Value != nil || v.Text != "–" {
				t.Errorf("%s: got %+v before the first response", it.key, v)
			}
		}
	}
}

func TestPage(t *testing.T) {
	srv := testServer(t, testRegistry(t, true, "Komfortbetrieb"))
	resp, body := get(t, srv.URL+"/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	for _, want := range []string{
		`<strong id="mode">Komfortbetrieb</strong>`,
		`data-key="tank_setpoint"`,
		`46,8 °C`,
		`href="/metrics"`,
		`/static/app.js?v=abc1234`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("Content-Security-Policy %q", csp)
	}
}

func TestValuesEndpoint(t *testing.T) {
	srv := testServer(t, testRegistry(t, true, "Komfortbetrieb"))
	resp, body := get(t, srv.URL+"/api/v1/values")
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type %q", ct)
	}
	var v valuesResponse
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	if v.Mode != "Komfortbetrieb" {
		t.Errorf("mode %q", v.Mode)
	}
	if it := v.Items["tank_comfort"]; it.Value == nil || *it.Value != 48 || it.Text != "48,0 °C" {
		t.Errorf("tank_comfort %+v", it)
	}
	if it := v.Items["flow"]; it.Value != nil || it.Text != "–" {
		t.Errorf("flow without metric: %+v", it)
	}
}

func TestStatic(t *testing.T) {
	srv := testServer(t, testRegistry(t, true, "Komfortbetrieb"))
	for _, path := range []string{"/static/style.css", "/static/app.js"} {
		if resp, _ := get(t, srv.URL+path); resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
	}
}
