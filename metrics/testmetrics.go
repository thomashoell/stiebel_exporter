package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/thomashoell/stiebel_exporter/stiebel"
)

// testIndexes are Elster indexes that are probably useful for an air-to-water heat pump (WPL) with a heat pump
// manager (WPM), but not verified on the real device yet: it is unknown which unit answers them and whether the
// Elster type (and therefore the scaling) from the can_progs table is right. They are exported as
// test_stiebel_register_value so they can be compared with the heat pump's display before they get proper metrics.
var testIndexes = []uint16{
	0x0059, // HEIZKREIS_STATUS, bit field
	0x005A, // SPEICHER_STATUS, bit field
	0x01D2, // PUMPENSTATUS, bit field
	0x0074, // EVU_SPERRE_AKTIV, 1 = blocked or released?
	0x01DA, // VOLUMENSTROM
	0x07A6, // ANZEIGE_HOCHDRUCK, probably bar * 100 instead of bar * 10
	0x07A7, // ANZEIGE_NIEDERDRUCK, probably bar * 100 instead of bar * 10
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
