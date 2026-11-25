package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/thomashoell/stiebel_exporter/stiebel"
)

// testIndexes are Elster indexes that are probably useful for an air-to-water heat pump (WPL) with a heat pump
// manager (WPM), but not verified on the real device yet: it is unknown which unit answers them and whether the
// Elster type (and therefore the scaling) from the can_progs table is right. They are exported as
// test_stiebel_register_value so they can be compared with the heat pump's display before they get proper metrics.
//
// Evaluated 2026-11-24 (docs/heat-pump-registers.md): 0x063D and 0x07A8 became regular metrics; 0x0062, 0x0068,
// 0x0070 don't answer, 0x063C and 0x0640 carry no information. Still open: what 0x063F and 0x033D (rising while
// the compressor stands still) and the single bits of 0x0176 mean.
//
// 2026-11-25: which registers hold the hot water set points of Komfortbetrieb and Eco-Betrieb (+10 … +60 °C on the
// panel)? Compare with the values set on the panel; SPEICHERSOLLTEMP (0x0003) is the effective one.
var testIndexes = []uint16{
	0x0013, // EINSTELL_SPEICHERSOLLTEMP
	0x0A06, // EINSTELL_SPEICHERSOLLTEMP2
	0x013E, // EINSTELL_SPEICHERSOLLTEMP3
	0x0133, // WW_SOLLWERT_REDUZIERT
	0x0118, // ECO_BETRIEB
	0x027E, // WW_ECO
	0x0176, // BETRIEBS_STATUS (bit field, 65 during a heating cycle)
	0x033D, // INTEGRAL_REGELABWEICHUNG
	0x063F, // ISTDREHZAHL_VERDICHTER_2 (not the speed, no regular metric any more)
}

// testUnits are the units asked for the test indexes; the manager is not polled for the regular metrics.
var testUnits = []string{"boiler", "heater", "manager"}

func newTestValues() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_stiebel_register_value",
		Help: "Experimental: decoded value of an Elster index, scaled according to its type in the can_progs " +
			"table. Not verified, names and scaling may be wrong.",
	}, []string{"unit", "index", "name"})
}

// addTestIndexes registers setters for the test indexes the unit doesn't export already
// and adds them to the unit's test poll list.
func (u *unit) addTestIndexes(values *prometheus.GaugeVec) {
	for _, index := range testIndexes {
		if _, exported := u.setters[index]; exported {
			continue
		}
		name := ""
		if r := stiebel.Reading(index); r != nil {
			name = r.Name
		}
		labels := []string{u.name, formatIndex(index), name}
		// the series is created with the first value, so indexes a unit doesn't know never show up
		u.setters[index] = func(v float64) { values.WithLabelValues(labels...).Set(v) }
		u.testPolled = append(u.testPolled, index)
	}
}
