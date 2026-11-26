package web

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Freshness limit: the exporter polls every 10 s by default, so a unit that hasn't answered for a minute is stale.
const staleAfter = time.Minute

// snapshot holds the current values of the registry, collected once per request.
type snapshot struct {
	now    time.Time
	gauges map[string]float64
	mode   string
	// energy sums the parts (magnitudes, heat generators) per metric and time frame, key "<metric>/<time_frame>"
	energy map[string]float64
	// lastResponse is the time of the last response per unit (boiler, heater, manager)
	lastResponse map[string]time.Time
}

func collect(g prometheus.Gatherer, now time.Time) (*snapshot, error) {
	mfs, err := g.Gather()
	if err != nil {
		return nil, fmt.Errorf("gather metrics: %w", err)
	}
	s := &snapshot{
		now:          now,
		gauges:       map[string]float64{},
		energy:       map[string]float64{},
		lastResponse: map[string]time.Time{},
	}
	for _, mf := range mfs {
		name := mf.GetName()
		for _, m := range mf.GetMetric() {
			if m.GetGauge() == nil {
				continue
			}
			v := m.GetGauge().GetValue()
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			switch name {
			case "stiebel_operating_mode":
				s.mode = labels["mode"]
			case "stiebel_electrical_power_consumption_watt_hours", "stiebel_heat_produced_watt_hours":
				s.energy[name+"/"+labels["time_frame"]] += v
			case "stiebel_exporter_last_response_timestamp_seconds":
				sec, frac := math.Modf(v)
				s.lastResponse[labels["unit"]] = time.Unix(int64(sec), int64(frac*1e9))
			default:
				if len(labels) == 0 {
					s.gauges[name] = v
				}
			}
		}
	}
	return s, nil
}

// answered reports whether the unit has answered at least once; before that its gauges are 0 and meaningless.
func (s *snapshot) answered(unit string) bool {
	_, ok := s.lastResponse[unit]
	return ok
}

// gauge returns the value of an unlabeled gauge of the unit.
func (s *snapshot) gauge(unit, name string) (float64, bool) {
	if !s.answered(unit) {
		return 0, false
	}
	v, ok := s.gauges[name]
	return v, ok
}

func (s *snapshot) energyOf(metric, timeFrame string) (float64, bool) {
	if !s.answered("boiler") {
		return 0, false
	}
	v, ok := s.energy[metric+"/"+timeFrame]
	return v, ok
}

// tankSetpoint is the effective hot water set point: the Eco one in Eco-Betrieb, the Komfort one otherwise.
// In Programmbetrieb the WPM switches by its time program, which isn't visible on the bus – Komfort is shown then.
func (s *snapshot) tankSetpoint() (float64, bool) {
	if s.mode == "Eco-Betrieb" {
		return s.gauge("boiler", "stiebel_boiler_tank_eco_setpoint_celsius")
	}
	return s.gauge("boiler", "stiebel_boiler_tank_comfort_setpoint_celsius")
}

// number formats v with German decimal separator.
func number(v float64, decimals int) string {
	if v == 0 {
		v = 0 // no "-0"
	}
	return strings.Replace(strconv.FormatFloat(v, 'f', decimals, 64), ".", ",", 1)
}

func celsius(v float64) (string, string) { return number(v, 1) + " °C", "" }
func bar(v float64) (string, string)     { return number(v, 1) + " bar", "" }
func percent(v float64) (string, string) { return number(v, 0) + " %", "" }
func count(v float64) (string, string)   { return number(v, 0), "" }

func energy(wh float64) (string, string) {
	if wh >= 1e6 {
		return number(wh/1e6, 2) + " MWh", ""
	}
	return number(wh/1e3, 1) + " kWh", ""
}

func duration(seconds float64) (string, string) {
	s := int(math.Round(seconds))
	return fmt.Sprintf("%d:%02d min", s/60, s%60), ""
}

func stages(v float64) (string, string) {
	if v == 0 {
		return "aus", "off"
	}
	return number(v, 0) + " von 3", "warn"
}

func yesNo(v float64) (string, string) {
	if v != 0 {
		return "ja", "on"
	}
	return "nein", "off"
}

// compressor states, see compressorState
const (
	compressorStopped = 0
	compressorRunning = 1
	compressorDefrost = 2
)

func compressorText(v float64) (string, string) {
	switch v {
	case compressorDefrost:
		return "Abtauung", "warn"
	case compressorRunning:
		return "läuft", "on"
	default:
		return "steht", "off"
	}
}

func (s *snapshot) compressorState() (float64, bool) {
	running, ok := s.gauge("heater", "stiebel_compressor_running")
	if !ok {
		return 0, false
	}
	if defrost, _ := s.gauge("heater", "stiebel_heater_defrost_active"); defrost != 0 {
		return compressorDefrost, true
	}
	if running != 0 {
		return compressorRunning, true
	}
	return compressorStopped, true
}

// age returns the seconds since the last response of the unit.
func (s *snapshot) age(unit string) (float64, bool) {
	t, ok := s.lastResponse[unit]
	if !ok {
		return 0, false
	}
	return s.now.Sub(t).Seconds(), true
}

func ageText(seconds float64) (string, string) {
	state := "on"
	if seconds > staleAfter.Seconds() {
		state = "warn"
	}
	switch {
	case seconds < 60:
		return fmt.Sprintf("vor %d s", int(math.Max(seconds, 0))), state
	case seconds < 3600:
		return fmt.Sprintf("vor %d min", int(seconds/60)), state
	default:
		return fmt.Sprintf("vor %d h", int(seconds/3600)), state
	}
}
