// Package web serves a small web UI with the current values of the heat pump. The values come from the Prometheus
// registry of the exporter, so the UI causes no additional requests on the CAN bus. With a ModeSetter, the UI can
// also change the operating mode.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"slices"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

//go:embed templates static
var content embed.FS

// Info describes the running exporter.
type Info struct {
	Version string
	Commit  string
}

// ModeSetter changes the operating mode of the heat pump.
type ModeSetter interface {
	// OperatingModes returns the names of the operating modes that may be set.
	OperatingModes() []string
	// SetOperatingMode sets the operating mode and returns the name read back from the heat pump.
	SetOperatingMode(ctx context.Context, name string) (string, error)
}

// setTimeout bounds a write including waiting for a running one; below the write timeout of the HTTP server.
const setTimeout = 12 * time.Second

// item is one value shown in the UI.
type item struct {
	key, label string
	get        func(s *snapshot) (float64, bool)
	format     func(float64) (text, state string)
}

type section struct {
	title string
	items []item
}

func gauge(unit, name string) func(*snapshot) (float64, bool) {
	return func(s *snapshot) (float64, bool) { return s.gauge(unit, name) }
}

func energyOf(metric, timeFrame string) func(*snapshot) (float64, bool) {
	return func(s *snapshot) (float64, bool) { return s.energyOf(metric, timeFrame) }
}

func age(unit string) func(*snapshot) (float64, bool) {
	return func(s *snapshot) (float64, bool) { return s.age(unit) }
}

const (
	electrical = "stiebel_electrical_power_consumption_watt_hours"
	heat       = "stiebel_heat_produced_watt_hours"
)

// overview holds the values of the tiles at the top (see hero); they have no section of their own.
var overview = []item{
	{"outdoor", "Außentemperatur", gauge("boiler", "stiebel_outdoor_temperature_celsius"), celsius},
	{"tank", "Warmwasser", gauge("boiler", "stiebel_boiler_tank_temperature_celsius"), celsius},
	{"compressor", "Verdichter", (*snapshot).compressorState, compressorText},
	{"modulation", "Modulation", gauge("heater", "stiebel_compressor_modulation_percent"), percent},
	{"booster", "Heizstäbe", gauge("heater", "stiebel_booster_heater_stages"), stages},
}

// sections defines the cards below the tiles; the labels follow the Grafana dashboard.
var sections = []section{
	{"Heizkreis", []item{
		{"flow", "Vorlauf", gauge("boiler", "stiebel_boiler_flow_temperature_celsius"), celsius},
		{"return", "Rücklauf", gauge("boiler", "stiebel_boiler_return_temperature_celsius"), celsius},
		{"flow_setpoint", "Vorlauf-Soll", gauge("boiler", "stiebel_heating_flow_setpoint_celsius"), celsius},
		{"heater_flow", "WP-Vorlauf", gauge("heater", "stiebel_heater_flow_temperature_celsius"), celsius},
		{"heater_return", "WP-Rücklauf", gauge("heater", "stiebel_heater_return_temperature_celsius"), celsius},
		{"room_setpoint", "Raum-Soll (Komfort)", gauge("boiler", "stiebel_room_comfort_setpoint_celsius"), celsius},
	}},
	{"Warmwasser", []item{
		{"tank_setpoint", "Soll (wirksam)", (*snapshot).tankSetpoint, celsius},
		{"tank_comfort", "Soll Komfort", gauge("boiler", "stiebel_boiler_tank_comfort_setpoint_celsius"), celsius},
		{"tank_eco", "Soll Eco", gauge("boiler", "stiebel_boiler_tank_eco_setpoint_celsius"), celsius},
		{"hot_water_charging", "Ladung aktiv", gauge("boiler", "stiebel_hot_water_charging"), yesNo},
	}},
	{"Kältekreis", []item{
		{"cycle", "Heizzyklus", gauge("heater", "stiebel_heating_cycle_active"), yesNo},
		{"hot_gas", "Heißgas", gauge("heater", "stiebel_heater_hot_gas_temperature_celsius"), celsius},
		{"high_pressure", "Hochdruck", gauge("heater", "stiebel_heater_high_pressure_bar"), bar},
		{"low_pressure", "Niederdruck", gauge("heater", "stiebel_heater_low_pressure_bar"), bar},
		{"fan", "Lüfter", gauge("heater", "stiebel_heater_fan_speed_percent"), percent},
	}},
	{"Abtauung", []item{
		{"defrost", "Abtauung aktiv", gauge("heater", "stiebel_heater_defrost_active"), yesNo},
		{"defrosts", "Abtauungen gesamt", gauge("heater", "stiebel_heater_defrosts"), count},
		{"last_defrost", "Letzte Abtaudauer", gauge("heater", "stiebel_heater_last_defrost_duration_seconds"), duration},
	}},
	{"Energie", []item{
		{"electrical_day", "Strom heute", energyOf(electrical, "day"), energy},
		{"electrical_total", "Strom gesamt", energyOf(electrical, "total"), energy},
		{"heat_day", "Wärme heute", energyOf(heat, "day"), energy},
		{"heat_total", "Wärme gesamt", energyOf(heat, "total"), energy},
	}},
	{"Verbindung", []item{
		{"age_boiler", "Kessel (WPM)", age("boiler"), ageText},
		{"age_heater", "Wärmepumpe", age("heater"), ageText},
	}},
}

// value is the JSON representation of an item; Value is null without data.
type value struct {
	Value *float64 `json:"value"`
	Text  string   `json:"text"`
	State string   `json:"state,omitempty"`
}

func (it item) evaluate(s *snapshot) value {
	v, ok := it.get(s)
	if !ok {
		return value{Text: "–"}
	}
	text, state := it.format(v)
	return value{Value: &v, Text: text, State: state}
}

type handler struct {
	gatherer prometheus.Gatherer
	info     Info
	logger   *slog.Logger
	page     *template.Template
	now      func() time.Time
	setter   ModeSetter
}

// Register adds the UI to mux: the page at /, its assets at /static/ and the current values at /api/v1/values.
// With a setter (may be nil), POST /api/v1/operating-mode changes the operating mode; requests from other origins
// are rejected.
func Register(mux *http.ServeMux, g prometheus.Gatherer, info Info, logger *slog.Logger, setter ModeSetter) {
	static, err := fs.Sub(content, "static")
	if err != nil {
		panic(err) // the embedded directory always exists
	}
	h := &handler{
		gatherer: g,
		info:     info,
		logger:   logger,
		page:     template.Must(template.ParseFS(content, "templates/index.html")),
		now:      time.Now,
		setter:   setter,
	}
	mux.Handle("GET /static/", secure(http.StripPrefix("/static/", http.FileServerFS(static))))
	mux.Handle("GET /{$}", secure(http.HandlerFunc(h.servePage)))
	mux.Handle("GET /api/v1/values", secure(http.HandlerFunc(h.serveValues)))
	if setter != nil {
		mux.Handle("POST /api/v1/operating-mode",
			secure(http.NewCrossOriginProtection().Handler(http.HandlerFunc(h.setOperatingMode))))
	}
}

// secure sets headers that keep the page from loading anything but its own assets.
func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

type pageItem struct {
	Key, Label, Text, State string
}

type pageSection struct {
	Title string
	Items []pageItem
}

// hero lists the values shown as large tiles at the top, each with an optional secondary value.
var hero = []struct{ key, sub, subLabel string }{
	{"outdoor", "", ""},
	{"tank", "tank_setpoint", "Soll"},
	{"compressor", "modulation", "Modulation"},
	{"booster", "", ""},
}

type heroTile struct {
	Key, Label, Text, State   string
	SubKey, SubLabel, SubText string
}

type pageData struct {
	Info     Info
	Mode     string
	Hero     []heroTile
	Sections []pageSection
	// Modes are the operating modes that can be set; empty if the UI is read-only.
	Modes []string
	// ModeSettable is set if the current mode is one of Modes.
	ModeSettable bool
}

// allItems returns the items of the tiles and of all sections.
func allItems() []item {
	items := append([]item{}, overview...)
	for _, sec := range sections {
		items = append(items, sec.items...)
	}
	return items
}

// itemByKey returns the item definition with the key.
func itemByKey(key string) (item, bool) {
	for _, it := range allItems() {
		if it.key == key {
			return it, true
		}
	}
	return item{}, false
}

func (h *handler) servePage(w http.ResponseWriter, _ *http.Request) {
	s, err := collect(h.gatherer, h.now())
	if err != nil {
		h.logger.Warn("cannot render web UI", "error", err)
		http.Error(w, "cannot collect values", http.StatusInternalServerError)
		return
	}
	data := pageData{Info: h.info, Mode: s.mode}
	if h.setter != nil {
		data.Modes = h.setter.OperatingModes()
		data.ModeSettable = slices.Contains(data.Modes, s.mode)
	}
	for _, t := range hero {
		it, ok := itemByKey(t.key)
		if !ok {
			continue
		}
		v := it.evaluate(s)
		tile := heroTile{Key: it.key, Label: it.label, Text: v.Text, State: v.State}
		if subItem, ok := itemByKey(t.sub); ok {
			tile.SubKey, tile.SubLabel, tile.SubText = subItem.key, t.subLabel, subItem.evaluate(s).Text
		}
		data.Hero = append(data.Hero, tile)
	}
	for _, sec := range sections {
		ps := pageSection{Title: sec.title}
		for _, it := range sec.items {
			v := it.evaluate(s)
			ps.Items = append(ps.Items, pageItem{Key: it.key, Label: it.label, Text: v.Text, State: v.State})
		}
		data.Sections = append(data.Sections, ps)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.page.Execute(w, data); err != nil {
		h.logger.Warn("cannot render web UI", "error", err)
	}
}

type valuesResponse struct {
	Time  time.Time        `json:"time"`
	Mode  string           `json:"mode"`
	Items map[string]value `json:"items"`
}

func (h *handler) serveValues(w http.ResponseWriter, _ *http.Request) {
	s, err := collect(h.gatherer, h.now())
	if err != nil {
		h.logger.Warn("cannot collect values for the web UI", "error", err)
		http.Error(w, "cannot collect values", http.StatusInternalServerError)
		return
	}
	resp := valuesResponse{Time: s.now, Mode: s.mode, Items: map[string]value{}}
	for _, it := range allItems() {
		resp.Items[it.key] = it.evaluate(s)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Warn("cannot write values", "error", err)
	}
}

type modeRequest struct {
	Mode string `json:"mode"`
}

type modeResponse struct {
	Mode  string `json:"mode,omitempty"`
	Error string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// setOperatingMode handles POST /api/v1/operating-mode with a JSON body {"mode": "<name>"}.
func (h *handler) setOperatingMode(w http.ResponseWriter, r *http.Request) {
	if ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || ct != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, modeResponse{Error: "content type must be application/json"})
		return
	}
	var req modeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, modeResponse{Error: "invalid request"})
		return
	}
	if !slices.Contains(h.setter.OperatingModes(), req.Mode) {
		writeJSON(w, http.StatusBadRequest, modeResponse{Error: "operating mode cannot be set"})
		return
	}
	logger := h.logger.With("mode", req.Mode, "remote", r.RemoteAddr, "forwarded_for", r.Header.Get("X-Forwarded-For"))
	logger.Info("operating mode change requested in the web UI")
	ctx, cancel := context.WithTimeout(r.Context(), setTimeout)
	defer cancel()
	mode, err := h.setter.SetOperatingMode(ctx, req.Mode)
	if err != nil {
		logger.Warn("cannot set the operating mode", "error", err)
		writeJSON(w, http.StatusBadGateway, modeResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, modeResponse{Mode: mode})
}
