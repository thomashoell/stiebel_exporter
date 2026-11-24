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
// Current round: what does the compressor do? ISTDREHZAHL_VERDICHTER_2 (0x063F, heater) is not the compressor
// speed on this device – it rises while the compressor stands still (hot gas cooling down) and goes negative while
// it runs – so these candidates are compared with the hot gas temperature to find a register that shows whether
// (and how fast) the compressor runs.
var testIndexes = []uint16{
	0x0062, // WAERMEPUMPEN_STATUS (bit field, little endian)
	0x0068, // BETRIEBSART
	0x0070, // DREHZAHLREG_JA_NEIN
	0x0176, // BETRIEBS_STATUS
	0x033D, // INTEGRAL_REGELABWEICHUNG (heating integral? 0x063F behaves like one)
	0x063C, // SOLLDREHZAHL_VERDICHTER_1 (always 0 on the heater, asked from boiler and manager too)
	0x063D, // SOLLDREHZAHL_VERDICHTER_2
	0x063F, // ISTDREHZAHL_VERDICHTER_2 (exported for the heater already, compare with boiler and manager)
	0x0640, // ZWISCHENEINSPRITZUNG_ISTTEMP_VND
	0x07A8, // VERDICHTER (dec)
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
