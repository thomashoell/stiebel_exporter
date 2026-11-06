package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/thomashoell/stiebel_exporter/stiebel"
)

// Metric names and labels are used by the Grafana dashboards, keep them stable.
const (
	namespace = "stiebel"

	// label values
	day         = "day"
	total       = "total"
	wh          = "WH"
	kwh         = "kWH"
	mwh         = "MWH"
	water       = "water"
	heat        = "heat"
	we1         = "1"
	we2         = "2"
	compressor1 = "1"
	compressor2 = "2"
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
	compressorSpeedRpm      *prometheus.GaugeVec
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
		compressorSpeedRpm: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "compressor_speed_rpm",
			Help:      "The compressor's revolutions per minute.",
		}, []string{"unit"}),
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
		g.compressorSpeedRpm,
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
	boiler.add(true, energyIndexes(0x091A, g.electricalPowerConsumed)...) // EL_AUFNAHMELEISTUNG_*
	boiler.add(false, energyIndexes(0x0922, g.heatProduced, we2)...)      // WAERMEERTRAG_2WE_* (polled from the heater)
	boiler.add(true, energyIndexes(0x092A, g.heatProduced, we1)...)       // WAERMEERTRAG_*

	heater := newUnit("heater", stiebel.Heizmodul)
	heater.add(true, energyIndexes(0x0922, g.heatProduced, we2)...) // WAERMEERTRAG_2WE_* (booster heater)
	heater.add(true,
		indexSetter{0x063E, set(g.compressorSpeedRpm.WithLabelValues(compressor1))}, // ISTDREHZAHL_VERDICHTER_1
		indexSetter{0x063F, set(g.compressorSpeedRpm.WithLabelValues(compressor2))}, // ISTDREHZAHL_VERDICHTER_2
		indexSetter{0x0265, set(g.hotGasTemperature)},                               // HEISSGAS_TEMP
	)

	// the manager is not polled, its responses are only logged
	manager := newUnit("manager", stiebel.Manager)

	return []*unit{boiler, heater, manager}
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
