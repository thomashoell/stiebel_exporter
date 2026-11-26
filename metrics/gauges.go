package metrics

import (
	"math"
	"math/bits"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/thomashoell/stiebel_exporter/stiebel"
)

// Metric names and labels are used by the Grafana dashboards, keep them stable.
const (
	namespace = "stiebel"

	// label values
	day   = "day"
	total = "total"
	wh    = "WH"
	kwh   = "kWH"
	mwh   = "MWH"
	water = "water"
	heat  = "heat"
	we1   = "1"
	we2   = "2"
)

// gauges holds the metrics with values read from the heat pump.
type gauges struct {
	boilerFlowTemperature   prometheus.Gauge
	boilerReturnTemperature prometheus.Gauge
	outdoorTemperature      prometheus.Gauge
	tankTemperature         prometheus.Gauge
	hotGasTemperature       prometheus.Gauge
	electricalPowerConsumed *prometheus.GaugeVec
	heatProduced            *prometheus.GaugeVec
	compressorModulation    prometheus.Gauge
	compressorRunning       prometheus.Gauge
	boosterHeaterStages     prometheus.Gauge

	heatingFlowSetpoint     prometheus.Gauge
	tankSetpoint            prometheus.Gauge
	tankComfortSetpoint     prometheus.Gauge
	tankEcoSetpoint         prometheus.Gauge
	roomComfortSetpoint     prometheus.Gauge
	operatingMode           *prometheus.GaugeVec
	heaterFlowTemperature   prometheus.Gauge
	heaterReturnTemperature prometheus.Gauge
	fanSpeed                prometheus.Gauge
	heatingCycleActive      prometheus.Gauge
	defrostActive           prometheus.Gauge
	highPressure            prometheus.Gauge
	lowPressure             prometheus.Gauge
	hotWaterCharging        prometheus.Gauge
	defrosts                prometheus.Gauge
	lastDefrostDuration     prometheus.Gauge
}

func newGauges() *gauges {
	return &gauges{
		boilerFlowTemperature: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "boiler",
			Name:      "flow_temperature_celsius",
			Help:      "Flow temperature as reported by the boiler.",
		}),
		boilerReturnTemperature: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "boiler",
			Name:      "return_temperature_celsius",
			Help:      "Return temperature as reported by the boiler.",
		}),
		outdoorTemperature: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "outdoor_temperature_celsius",
			Help:      "Temperature measured by the external sensor.",
		}),
		tankTemperature: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "boiler",
			Name:      "tank_temperature_celsius",
			Help:      "Temperature of the water in the tank as reported by the boiler.",
		}),
		hotGasTemperature: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "hot_gas_temperature_celsius",
			Help:      "Hot gas temperature.",
		}),
		electricalPowerConsumed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "electrical_power_consumption_watt_hours",
			Help:      "The total amount of electrical power consumed.",
		}, []string{"time_frame", "magnitude", "component"}),
		heatProduced: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "heat_produced_watt_hours",
			Help:      "The total amount of heat generated.",
		}, []string{"time_frame", "magnitude", "component", "heat_generator"}),
		compressorModulation: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "compressor_modulation_percent",
			Help: "Modulation of the inverter compressor in percent (SOLLDREHZAHL_VERDICHTER_2): 0 while it stands " +
				"still (also at the start of a defrost), otherwise from the lowest stage (~28) to 100.",
		}),
		compressorRunning: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "compressor_running",
			Help:      "1 while the compressor runs (bit 0x10 of VERDICHTER), including the reverse-cycle part of a defrost.",
		}),
		boosterHeaterStages: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "booster_heater_stages",
			Help: "Number of active electric heating elements of the booster heater (bits 0x40/0x80 of VERDICHTER as " +
				"a 2-bit count, 0-3; one element per phase, ~2.9 kW each on the reference device).",
		}),

		heatingFlowSetpoint: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "heating_flow_setpoint_celsius",
			Help:      "Set point of the heating flow temperature (KESSELSOLLTEMP), from the heating curve.",
		}),
		tankSetpoint: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "boiler",
			Name:      "tank_setpoint_celsius",
			Help: "Hot water set point as reported in SPEICHERSOLLTEMP; not the effective one, it keeps the Komfort " +
				"value in Eco-Betrieb.",
		}),
		tankComfortSetpoint: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "boiler",
			Name:      "tank_comfort_setpoint_celsius",
			Help:      "Hot water set point of Komfortbetrieb (EINSTELL_SPEICHERSOLLTEMP).",
		}),
		tankEcoSetpoint: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "boiler",
			Name:      "tank_eco_setpoint_celsius",
			Help:      "Hot water set point of Eco-Betrieb (EINSTELL_SPEICHERSOLLTEMP2).",
		}),
		roomComfortSetpoint: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "room_comfort_setpoint_celsius",
			Help:      "Room temperature set point for comfort (day) mode (RAUMSOLLTEMP_I).",
		}),
		operatingMode: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "operating_mode",
			Help:      "Operating mode selected with the program switch (PROGRAMMSCHALTER); the value is the raw code.",
		}, []string{"mode"}),
		heaterFlowTemperature: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "flow_temperature_celsius",
			Help:      "Flow temperature measured in the heat pump (VORLAUFISTTEMP_WP_IWS).",
		}),
		heaterReturnTemperature: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "return_temperature_celsius",
			Help:      "Return temperature measured in the heat pump (RUECKLAUFISTTEMP_WP_IWS).",
		}),
		fanSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "fan_speed_percent",
			Help:      "Speed of the evaporator fan in percent (LUEFTERDREHZAHL).",
		}),
		heatingCycleActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "heating_cycle_active",
			Help: "1 from the start of the compressor to its stop, including the defrosts in between (bit 0x0200 " +
				"of ABTAUUNGAKTIV), 0 while the compressor stands still.",
		}),
		defrostActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "defrost_active",
			Help:      "1 while the evaporator is defrosted (bit 0x100 of VERDICHTER), 0 otherwise.",
		}),
		highPressure: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "high_pressure_bar",
			Help:      "High pressure of the refrigerant circuit (ANZEIGE_HOCHDRUCK).",
		}),
		lowPressure: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "low_pressure_bar",
			Help:      "Low pressure of the refrigerant circuit (ANZEIGE_NIEDERDRUCK).",
		}),
		hotWaterCharging: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "hot_water_charging",
			Help:      "1 while the hot water tank is charged (bit 0x4000 of PUMPENSTATUS), 0 otherwise.",
		}),
		defrosts: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "defrosts",
			Help:      "Number of defrosts as counted by the heat pump (STARTS_ABTAUUNG).",
		}),
		lastDefrostDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "heater",
			Name:      "last_defrost_duration_seconds",
			Help:      "Duration of the last defrost (ZEITDAUER_LETZTE_ABTAUUNG); the unit is assumed to be seconds.",
		}),
	}
}

func (g *gauges) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		g.boilerFlowTemperature,
		g.boilerReturnTemperature,
		g.outdoorTemperature,
		g.tankTemperature,
		g.hotGasTemperature,
		g.electricalPowerConsumed,
		g.heatProduced,
		g.compressorModulation,
		g.compressorRunning,
		g.boosterHeaterStages,
		g.heatingFlowSetpoint,
		g.tankSetpoint,
		g.tankComfortSetpoint,
		g.tankEcoSetpoint,
		g.roomComfortSetpoint,
		g.operatingMode,
		g.heaterFlowTemperature,
		g.heaterReturnTemperature,
		g.fanSpeed,
		g.heatingCycleActive,
		g.defrostActive,
		g.highPressure,
		g.lowPressure,
		g.hotWaterCharging,
		g.defrosts,
		g.lastDefrostDuration,
	}
}

// setter stores a decoded value in a metric.
type setter func(float64)

func set(g prometheus.Gauge) setter {
	return g.Set
}

// energyParts describes four consecutive Elster indexes that together make up an energy value:
// the Wh and kWh of the current day and the kWh and MWh of the total. Each part is exported in Wh.
var energyParts = []struct {
	timeFrame, magnitude string
	factor               float64
}{
	{day, wh, 1},
	{day, kwh, 1e3},
	{total, kwh, 1e3},
	{total, mwh, 1e6},
}

// energyIndexes returns the setters for the eight consecutive Elster indexes starting at first:
// four for hot water, followed by four for heating. extraLabels are appended to the label values.
func energyIndexes(first uint16, vec *prometheus.GaugeVec, extraLabels ...string) []indexSetter {
	setters := make([]indexSetter, 0, 2*len(energyParts))
	index := first
	for _, component := range []string{water, heat} {
		for _, part := range energyParts {
			labels := append([]string{part.timeFrame, part.magnitude, component}, extraLabels...)
			g := vec.WithLabelValues(labels...)
			factor := part.factor
			setters = append(setters, indexSetter{index, func(v float64) { g.Set(v * factor) }})
			index++
		}
	}
	return setters
}

type indexSetter struct {
	index uint16
	set   setter
}

// unit is a bus participant whose values are exported.
type unit struct {
	name string
	id   uint16
	// polled lists the Elster indexes requested from this unit in every poll cycle.
	polled []uint16
	// setters maps the Elster indexes in responses of this unit to metrics. Responses to indexes that are
	// not polled are exported too, e.g. if another participant (ComfortSoft) requested them.
	setters map[uint16]setter
	// testPolled lists the test indexes requested from this unit, see testIndexes.
	testPolled []uint16
}

func newUnits(g *gauges) []*unit {
	boiler := newUnit("boiler", stiebel.Kessel)
	boiler.add(true,
		indexSetter{0x0016, set(g.boilerReturnTemperature)}, // RUECKLAUFISTTEMP
		indexSetter{0x01D6, set(g.boilerFlowTemperature)},   // WPVORLAUFIST
		indexSetter{0x000C, set(g.outdoorTemperature)},      // AUSSENTEMP
		indexSetter{0x000E, set(g.tankTemperature)},         // SPEICHERISTTEMP
	)
	boiler.add(true,
		indexSetter{0x0002, set(g.heatingFlowSetpoint)},         // KESSELSOLLTEMP
		indexSetter{0x0003, set(g.tankSetpoint)},                // SPEICHERSOLLTEMP
		indexSetter{0x0013, set(g.tankComfortSetpoint)},         // EINSTELL_SPEICHERSOLLTEMP
		indexSetter{0x0A06, set(g.tankEcoSetpoint)},             // EINSTELL_SPEICHERSOLLTEMP2
		indexSetter{0x0005, set(g.roomComfortSetpoint)},         // RAUMSOLLTEMP_I
		indexSetter{0x0112, setOperatingMode(g.operatingMode)},  // PROGRAMMSCHALTER
		indexSetter{0x01D2, setBit(g.hotWaterCharging, 0x4000)}, // PUMPENSTATUS
	)
	boiler.add(true, energyIndexes(0x091A, g.electricalPowerConsumed)...) // EL_AUFNAHMELEISTUNG_*
	boiler.add(false, energyIndexes(0x0922, g.heatProduced, we2)...)      // WAERMEERTRAG_2WE_* (polled from the heater)
	boiler.add(true, energyIndexes(0x092A, g.heatProduced, we1)...)       // WAERMEERTRAG_*

	heater := newUnit("heater", stiebel.Heizmodul)
	heater.add(true, energyIndexes(0x0922, g.heatProduced, we2)...) // WAERMEERTRAG_2WE_* (booster heater)
	heater.add(true,
		// SOLLDREHZAHL_VERDICHTER_2; ISTDREHZAHL_VERDICHTER_1/2 (0x063E/0x063F) are not the compressor speed on a
		// WPM, see docs/heat-pump-registers.md
		indexSetter{0x063D, set(g.compressorModulation)},
		// VERDICHTER, bit field: 0x10 = compressor running, 0x40/0x80 = number of heating elements, 0x100 = defrost
		indexSetter{0x07A8, setAll(
			setBit(g.compressorRunning, 0x0010),
			setBits(g.boosterHeaterStages, 0x00C0),
			setBit(g.defrostActive, 0x0100),
		)},
		indexSetter{0x0265, set(g.hotGasTemperature)}, // HEISSGAS_TEMP
	)
	heater.add(true,
		indexSetter{0x0822, set(g.heaterFlowTemperature)},         // VORLAUFISTTEMP_WP_IWS
		indexSetter{0x0823, set(g.heaterReturnTemperature)},       // RUECKLAUFISTTEMP_WP_IWS
		indexSetter{0x0833, set(g.fanSpeed)},                      // LUEFTERDREHZAHL
		indexSetter{0x0061, setBit(g.heatingCycleActive, 0x0200)}, // ABTAUUNGAKTIV
		indexSetter{0x07A6, setDivided(g.highPressure, 10)},       // ANZEIGE_HOCHDRUCK, decoded value is bar * 10
		indexSetter{0x07A7, setDivided(g.lowPressure, 10)},        // ANZEIGE_NIEDERDRUCK, decoded value is bar * 10
		indexSetter{0x0806, set(g.defrosts)},                      // STARTS_ABTAUUNG
		indexSetter{0x0807, set(g.lastDefrostDuration)},           // ZEITDAUER_LETZTE_ABTAUUNG
	)

	// the manager is not polled, its responses are only logged
	manager := newUnit("manager", stiebel.Manager)

	return []*unit{boiler, heater, manager}
}

// setBit sets g to 1 if the bit in mask is set in the 16 bit register value, to 0 otherwise.
func setBit(g prometheus.Gauge, mask uint16) setter {
	return func(v float64) {
		if uint16(int16(v))&mask != 0 { //nolint:gosec // the decoded value is a 16 bit register
			g.Set(1)
		} else {
			g.Set(0)
		}
	}
}

// setAll passes the value to all setters, for registers with several metrics (bit fields).
func setAll(setters ...setter) setter {
	return func(v float64) {
		for _, s := range setters {
			s(v)
		}
	}
}

// setBits sets g to the bits in mask of the 16 bit register value, shifted down to start at bit 0.
func setBits(g prometheus.Gauge, mask uint16) setter {
	shift := bits.TrailingZeros16(mask)
	return func(v float64) {
		g.Set(float64(uint16(int16(v)) & mask >> shift)) //nolint:gosec // the decoded value is a 16 bit register
	}
}

// setDivided sets g to the value divided by divisor, rounded to 3 decimals to hide floating point artifacts.
func setDivided(g prometheus.Gauge, divisor float64) setter {
	return func(v float64) { g.Set(math.Round(v/divisor*1000) / 1000) }
}

// setOperatingMode exports the raw operating mode code with its name as label; only the current mode has a series.
func setOperatingMode(vec *prometheus.GaugeVec) setter {
	return func(v float64) {
		code := uint16(int16(v)) //nolint:gosec // the decoded value is a 16 bit register
		mode, ok := stiebel.OperatingModeName(code)
		if !ok {
			mode = "unknown"
		}
		vec.Reset()
		vec.WithLabelValues(mode).Set(v)
	}
}

func newUnit(name string, id uint16) *unit {
	return &unit{name: name, id: id, setters: map[uint16]setter{}}
}

// add registers setters for responses of the unit; with poll, their indexes are requested in every poll cycle.
func (u *unit) add(poll bool, setters ...indexSetter) {
	for _, s := range setters {
		u.setters[s.index] = s.set
		if poll {
			u.polled = append(u.polled, s.index)
		}
	}
}
