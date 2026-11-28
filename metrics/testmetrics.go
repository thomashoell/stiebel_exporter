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
	0x033D, // INTEGRAL_REGELABWEICHUNG – 0.1 K, superheat actual − set point (panel "Regelabweichung", 2026-11-28)
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
	// Registers the manager polls from 0x700 every 5 s (candump 2026-11-26); can_progs names don't fit, the values
	// look like temperatures in 0.1 °C. Indexes missing in the table are exported with an empty name.
	0xFDF3, // 36.2–37.x
	0xFDF4, // 28.0–28.3
	0xFDF5, // 36.5–37.7
	0xFE07, // 2.5–2.9 (second outdoor temperature?)
	0xFE09, // 21.5–21.8 (room temperature?)
	0xFE1B, // counts down 7 → 0
	0xFE1D, // 1
	0xFE1E, // 0
	0xFE4C, // 18
	// 2026-11-26: booster heater settings, compared with the panel (Einstellungen › Heizen/Warmwasser › Elektrische
	// Nacherwärmung): bivalence −5 °C heating / −2 °C hot water, lower limit −20 °C, 3 stages, delay 60 min
	// Verified 2026-11-26: 0x01AC –5, 0x01AD –2, 0x01AE/0x01AF –20 (boiler and manager); 0x05A0 answers 60 = the
	// delay, not the number of stages (the table is shifted by one here); 0x05A1, 0x0589, 0x058B, 0x05A8 and
	// 0x05A9 don't answer.
	0x01AC, // BIVALENTPARALLELTEMPERATUR_HZG
	0x01AD, // BIVALENTPARALLELTEMPERATUR_WW
	0x01AE, // BIVALENZALTERNATIVTEMPERATUR_HZG
	0x01AF, // BIVALENZALTERNATIVTEMPERATUR_WW
	0x05A0, // "NHZ_ANZAHL_STUFEN", answers 60 = NHZ delay heating (min)
	0x059F, // "FUSSPUNKT_HEIZKURVE" – the number of stages (3) if the table is shifted by one?
	0x05A2, // "NHZ_LEISTUNG1"
	// 2026-11-28: panel menu Diagnose › Analyse Wärmepumpe – the panel (0x100) reads these from 0x514 (candump);
	// mapped to the panel by value: superheat control of the expansion valve, inverter temperatures, motor current.
	0x07F3, // SOLLWERT_UEBERHITZUNG (0.1 K, panel "Soll-Überhitzung")
	0x07F4, // ISTWERT_UEBERHITZUNG_VERDAMPFER (0.1 K, "Ist-Überhitzung-V")
	0x07F6, // VORSTEUER_OEFFNUNGSGRAD (0.1 %, "Vorsteuer-Öffnungsgrad EXV")
	0x07F7, // P_ANTEIL_EXV (0.01, "P-Faktor")
	0x07F8, // I_ANTEIL_EXV (0.01, "I-Faktor")
	0x07F9, // D_ANTEIL_EXV (0.01, "D-Faktor")
	0x07CA, // EXV_OEFFNUNGSGRAD (0.1 %, "Öffnungsgrad EXV")
	0xC1E6, // 0.1 °C, "Umgebungstemperatur INV"
	0xC1E7, // 0.1 °C, "Temperatur INV-Verdichter"
	0xC2C2, // 0.1 °C, "Temperatur INV-Lüfter"
	0xC2C3, // 0.1 %, "Öffnungsgrad Bypassventil"
	0x06B4, // "TEST_OBJEKT_115": 0.1 A, "Strom Motor"
}

// testUnits are the units asked for the test indexes; the manager is not polled for the regular metrics.
var testUnits = []string{"boiler", "heater", "manager", "0x100", "0x700", "0x514"}

// newTestUnits returns the bus participants that only exist for test metrics: seen on the bus (candump 2026-11-26)
// but not identified yet – 0x100 reads values from the boiler (probably the FES remote control), 0x700 answers the
// manager (probably the MFG multi-function group or the outdoor unit). They are named after their CAN ID until they
// are identified, and they are not asked: they don't answer requests, their responses to others are picked up.
// newPolledTestUnit returns 0x514: the panel reads the heat pump analysis values from it (candump 2026-11-28), so
// it answers requests; probably a second address of the heat module.
func newPolledTestUnit() *unit {
	return newUnit("0x514", 0x514)
}

func newTestUnits() []*unit {
	return []*unit{newUnit("0x100", 0x100), newUnit("0x700", stiebel.Fremdgeraet)}
}

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
