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
// 2026-11-25: the hot water set points of Komfort and Eco are 0x0013 and 0x0A06 (now regular metrics); 0x013E, 0x0133
// and 0x0118 don't answer. WW_ECO is still open (false in Eco-Betrieb).
var testIndexes = []uint16{
	0x0176, // BETRIEBS_STATUS (bit field, 65 during a heating cycle)
	0x033D, // INTEGRAL_REGELABWEICHUNG
	0x063F, // ISTDREHZAHL_VERDICHTER_2 (not the speed, no regular metric any more)
	0x027E, // WW_ECO
	// 2026-11-26: booster heater – bits 0x40/0x80 of 0x07A8 became stiebel_booster_heater_stages; 0x0137
	// KESSELSOLLTEMP_2WE doesn't answer. Still watched: the operating hours and the bit field of the pumps.
	0x01CB, // LAUFZEIT_2WE
	0x01D2, // PUMPENSTATUS (regular metric uses bit 0x4000; raw value, little endian)
	// 2026-11-26: which devices are on the bus? The panel lists WPM3 390-13, FES 417-07, MFG 18, WP1 446-04 and the
	// heat pump type WPL 17 ACS classic – look for software number/version, device ID and type per unit.
	0x000B, // GERAETE_ID
	0x0199, // SOFTWARE_NUMMER
	0x019A, // SOFTWARE_VERSION
	0x0148, // GERAETE_KENNUNG
	0x019C, // INFO_TYP
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

// addTestIndexes registers setters for the test indexes and adds the ones the unit doesn't export already to its
// test poll list. Indexes of regular metrics are not polled twice; their decoded value is exported in addition.
func (u *unit) addTestIndexes(values *prometheus.GaugeVec) {
	for _, index := range testIndexes {
		name := ""
		if r := stiebel.Reading(index); r != nil {
			name = r.Name
		}
		labels := []string{u.name, formatIndex(index), name}
		// the series is created with the first value, so indexes a unit doesn't know never show up
		test := func(v float64) { values.WithLabelValues(labels...).Set(v) }
		if regular, exported := u.setters[index]; exported {
			// already polled for a regular metric: also export the decoded value (e.g. all bits of a bit field)
			u.setters[index] = setAll(regular, test)
			continue
		}
		u.setters[index] = test
		u.testPolled = append(u.testPolled, index)
	}
}
