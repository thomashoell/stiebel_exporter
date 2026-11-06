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
	// temperatures and set points
	0x0002, // KESSELSOLLTEMP
	0x0003, // SPEICHERSOLLTEMP
	0x0004, // VORLAUFSOLLTEMP
	0x0005, // RAUMSOLLTEMP_I
	0x000D, // SAMMLERISTTEMP
	0x000F, // VORLAUFISTTEMP
	0x0011, // RAUMISTTEMP
	0x0014, // VERDAMPFERTEMP
	0x01D4, // QUELLE_IST
	0x0264, // TAUPUNKT_TEMP
	0x034F, // VERFLUESSIGERTEMP
	0x0822, // VORLAUFISTTEMP_WP_IWS
	0x0823, // RUECKLAUFISTTEMP_WP_IWS
	// pressures, flow, fan
	0x001F, // WASSERDRUCK
	0x07A6, // ANZEIGE_HOCHDRUCK
	0x07A7, // ANZEIGE_NIEDERDRUCK
	0x01DA, // VOLUMENSTROM
	0x0833, // LUEFTERDREHZAHL
	0x063C, // SOLLDREHZAHL_VERDICHTER_1
	// defrosting
	0x0061, // ABTAUUNGAKTIV
	0x0806, // STARTS_ABTAUUNG
	0x0807, // ZEITDAUER_LETZTE_ABTAUUNG
	// run times
	0x01C4, // LAUFZEIT_WP1
	0x05A5, // LAUFZEIT_VERDICHTER_HEIZEN
	0x05A6, // LAUFZEIT_VERDICHTER_KUEHLEN
	0x05A7, // LAUFZEIT_VERDICHTER_WW
	0x05A8, // LAUFZEIT_NHZ_WW
	0x05A9, // LAUFZEIT_NHZ_HEIZEN
	// status
	0x0059, // HEIZKREIS_STATUS
	0x005A, // SPEICHER_STATUS
	0x0062, // WAERMEPUMPEN_STATUS
	0x0074, // EVU_SPERRE_AKTIV
	0x0112, // PROGRAMMSCHALTER
	0x01D2, // PUMPENSTATUS
	// energy
	0xFDB8, // WAERMEMENGE
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
