# Heat pump registers – findings on the real device

What the Elster indexes mean on the heat pump this exporter was developed against, verified with test polls
(`-test-metrics`) and Prometheus data. Index names come from the can_progs table (`stiebel/readings.go`); their
meaning and scaling on a WPL/WPM often differ from the name. This document is kept up to date while registers are
investigated – open questions are marked **open**, interpretations that turned out wrong are kept (struck out or
noted) so the history stays traceable.

## Device

- Stiebel Eltron air-to-water heat pump (WPL, inverter compressor) with heat pump manager (WPM), read via CAN bus.
- Units (CAN IDs): boiler / WPM `0x180` ("Kessel"), heat module `0x500` ("Heizmodul", label `heater`),
  manager `0x480`. The exporter acts like ComfortSoft (`0x680`).
- Set points (0x0002, 0x0003, 0x0005, 0x0112) answer identically from boiler and manager.
- A response with value `0x8000` means "not available".

## Verified registers

| Index | Name | Unit | Meaning on this device |
|---|---|---|---|
| 0x0002 | KESSELSOLLTEMP | boiler | heating flow set point (heating curve), °C (dec) |
| 0x0003 | SPEICHERSOLLTEMP | boiler | hot water set point, °C – **not** the effective one: in Eco-Betrieb with an Eco hot water set point of 47 °C it stayed at 48 °C (2026-11-25), probably the Komfort set point (to be confirmed) |
| 0x0013 | EINSTELL_SPEICHERSOLLTEMP | boiler, manager | hot water set point of **Komfortbetrieb**, °C (dec); 48 °C as set on the panel (2026-11-25) |
| 0x0A06 | EINSTELL_SPEICHERSOLLTEMP2 | boiler, manager | hot water set point of **Eco-Betrieb**, °C (dec); 47 °C as set on the panel (2026-11-25) |
| 0x0005 | RAUMSOLLTEMP_I | boiler | room comfort set point, °C |
| 0x000C | AUSSENTEMP | boiler | outdoor temperature, °C |
| 0x0112 | PROGRAMMSCHALTER | boiler | operating mode, raw code: 0x0300 = 768 = **Komfortbetrieb** (verified on the WPM display, can_progs calls it Tagbetrieb); 0x0400 = 1024 = **Eco-Betrieb** (verified 2026-11-25, can_progs: Absenkbetrieb; the heating flow set point dropped from 35 to 32.1 °C, the hot water set point 0x0003 stayed at 48 °C) |
| 0x01D2 | PUMPENSTATUS | boiler | bit field, **little endian**: 0x4000 = hot water charging, 0x0100 = same as bit 0x0200 of 0x0061 (see below), 0x1000 and 0x0002 always set |
| 0x0265 | HEISSGAS_TEMP | heater | hot gas °C: ~100 °C while the compressor runs (up to 113 while charging hot water), decays exponentially towards ~50 °C when it stops – a good plausibility check for the compressor state |
| 0x0822/0x0823 | VORLAUF/RUECKLAUFISTTEMP_WP_IWS | heater | flow/return measured in the heat pump, °C |
| 0x0833 | LUEFTERDREHZAHL | heater | evaporator fan in **percent** (0–100), not rpm; 90–100 % while the compressor runs, pulses 0/30 % every ~4 min while it stands still |
| 0x07A6/0x07A7 | ANZEIGE_HOCH/NIEDERDRUCK | heater | refrigerant pressures, decoded (dec) value ÷ 10 = bar. HD 26–29 bar while heating, 32–37 bar while charging hot water, falls to ~19 bar when the compressor stops; ND 4–9 bar |
| 0x0061 | ABTAUUNGAKTIV | heater | bit field; bit 0x0200 = **heating cycle active**: set from compressor start to stop, *including* the defrosts inside the cycle (~~defrost phase~~, see investigation 2026-11-24); 0x0100 seen briefly (768) at cycle start |
| 0x063D | SOLLDREHZAHL_VERDICHTER_2 | heater | **compressor modulation in percent**: 0 while the compressor stands still and during defrosts, otherwise 28–100 (28 % = lowest stage, never above 100). Ramps up to 100 within ~3 min after a start at −4 °C, then modulates down; at 0 °C it stayed ≤ 76 %. Was 0 in the test polls of 2026-11-06/07 (taken while the compressor stood still) |
| 0x07A8 | VERDICHTER | heater | **bit field**, not a decimal value (the can_progs table says `TypeDecVal`, which divides by 10): 0x08 always set, **0x10 = compressor running**, **0x100 = defrost**. Sequence of every defrost: 0x18 → 0x08 (~1 min stop, presumably switching the 4-way valve) → 0x108 → 0x118 → 0x18 |
| 0x0806 | STARTS_ABTAUUNG | heater | defrost counter (~7 per day at −5 °C) |
| 0x0807 | ZEITDAUER_LETZTE_ABTAUUNG | heater | last defrost duration, seconds assumed (128–232) |

## Questionable registers

| Index | Name | Unit | Observation |
|---|---|---|---|
| 0x063F | ISTDREHZAHL_VERDICHTER_2 | heater | exported as `stiebel_compressor_speed_rpm{unit="2"}`, but **not the compressor speed** (see investigation 2026-11-24): rises while the compressor stands still, negative while it runs. Signed 16 bit, values on a grid of ~2.55 (0, 3, 5, 8, 10, 13, …), range −106 … 497. **open:** what it really is |
| 0x063E | ISTDREHZAHL_VERDICHTER_1 | heater | always 0 |
| 0x033D | INTEGRAL_REGELABWEICHUNG | heater | behaves like a heating integral: rises while the compressor stands still (to ~200–240), around 0 (−50 … +15) while it runs, spikes up to 674 during defrosts. No fixed start threshold (233, 194 before two starts). Scale/unit **open** |
| 0x0176 | BETRIEBS_STATUS | boiler | bit field: 65 (0x41) during a heating cycle including its defrosts, 0 while the compressor stands still (once dropped 1 min after the other signals). Meaning of the single bits **open** |

## Not usable on this device

- 0x063C SOLLDREHZAHL_VERDICHTER_1 – always 0.
- 0x0074 EVU_SPERRE_AKTIV – always 1 (heat pump running), probably "released"; no information.
- 0x01DA VOLUMENSTROM, 0x0059 HEIZKREIS_STATUS, 0x005A SPEICHER_STATUS (2 boiler / 129 heater), 0x01C4
  LAUFZEIT_WP1, 0xFDB8 WAERMEMENGE – constant.
- 0x0004 VORLAUFSOLLTEMP (5), 0x000F VORLAUFISTTEMP (−400 = sensor missing) – unused circuit.
- No answer to the hot water set point candidates 0x013E EINSTELL_SPEICHERSOLLTEMP3, 0x0133 WW_SOLLWERT_REDUZIERT,
  0x0118 ECO_BETRIEB (2026-11-25).
- No unit answered: room, evaporator, condenser, source temperatures, water pressure, compressor run times
  (0x000D, 0x0011, 0x0014, 0x01D4, 0x0264, 0x034F, 0x001F, 0x05A5–0x05A9).

## Investigations

### 2026-11-24: Does the compressor run continuously? What is 0x063F?

**Trigger:** the alert `HeatPumpCompressorStopped` (`max_over_time(stiebel_compressor_speed_rpm{unit="2"}[30m]) == 0`)
fired once at 05:56 at −5 °C outside. The night before every "defrost phase" had pushed 0x063F below zero; at 05:56
the 30 min window happened to hold only values ≤ 0 with a maximum of exactly 0.

**Assumption so far:** the inverter compressor runs continuously and only slows down (~70 rpm) during "defrost
phases" (bit 0x0200 of 0x0061, 16 min–5 h, ~38 % of the time in November).

**Observation (Prometheus, 2026-11-24 05:00–07:00, −5 °C):**

| Phase | Hot gas | High pressure | Fan | Flow | 0x063F |
|---|---|---|---|---|---|
| 04:57–05:24 (bit 0) | 89 → 51 °C, exponential decay | 25 → 19 bar | pulses 0/30 % | falls | rises 159 → 337 |
| 05:24–06:11 (bit 1) | jumps to ~100 °C | 26–29 bar | 90–100 % | 39 → 44 °C | negative, rises slowly to ~33 |
| 06:11–06:36 (bit 0) | 96 → 57 °C | falls | pulses | falls | rises 277 → 360 |

**Conclusions:**

- Bit 0x0200 of 0x0061 is set exactly while the compressor runs (hot gas ~100 °C, high pressure rising, fan at
  full speed, flow temperature rising). It is not a defrost phase. The actual defrosts (counter 0x0806 +1, high
  pressure drops to ~9 bar, low pressure up to 18 bar, fan off, short spike of 0x063F to 150–265) happen
  *inside* these phases.
- The compressor **cycles**: at −5 °C it runs ~45–90 min and stands still ~25–30 min.
- 0x063F is anti-correlated with compressor activity, so it is not an actual speed. At every compressor start it
  stood at ~330–360 – it might be a controller value such as the heating integral (**open**).
- The decoding in the exporter is correct (raw signed 16 bit big endian, `TypeNone`); the WPM itself sends the
  negative values.

**Next step:** test poll of candidate registers (`metrics/testmetrics.go`, snapshot 1.4.3-snapshot.bc37d5b on
daedalus with `-test-metrics` since 2026-11-24 09:14): 0x0062 WAERMEPUMPEN_STATUS, 0x0068 BETRIEBSART, 0x0070
DREHZAHLREG_JA_NEIN, 0x0176 BETRIEBS_STATUS, 0x033D INTEGRAL_REGELABWEICHUNG, 0x063C/0x063D SOLLDREHZAHL_VERDICHTER_1/2,
0x063F (from boiler and manager), 0x0640, 0x07A8 VERDICHTER.

First answers (09:14, compressor running):

| Index | Name | Unit | Value |
|---|---|---|---|
| 0x0176 | BETRIEBS_STATUS | boiler | bit field: 65 (0x41) during a heating cycle including its defrosts, 0 while the compressor stands still (once dropped 1 min after the other signals). Meaning of the single bits **open** |
| 0x033D | INTEGRAL_REGELABWEICHUNG | heater | behaves like a heating integral: rises while the compressor stands still (to ~200–240), around 0 (−50 … +15) while it runs, spikes up to 674 during defrosts. No fixed start threshold (233, 194 before two starts). Scale/unit **open** |
| 0x063C | SOLLDREHZAHL_VERDICHTER_1 | heater | 0 |
| 0x063D | SOLLDREHZAHL_VERDICHTER_2 | heater | 37 |
| 0x0640 | ZWISCHENEINSPRITZUNG_ISTTEMP_VND | heater / boiler, manager | 34 / 16 |
| 0x07A8 | VERDICHTER | heater | 2.4 |

No answer: 0x0062, 0x0068, 0x0070, 0x063F from boiler and manager.

First full cycle (09:15–10:27, −5 … −3 °C; compressor stops 09:18, starts 09:52, defrost 10:04 with counter +1):

| Time | Hot gas | Bit 0x0200 of 0x0061 | 0x063D | 0x0176 | 0x07A8 (raw) | 0x033D | 0x063F |
|---|---|---|---|---|---|---|---|
| 09:15 (running) | ~95 °C | 1 | 36 | 65 | 0x18 | −4 | 28 |
| 09:18 stop | falls to ~55 °C | 0 | 0 | 0 | 0x08 | rises to ~240 | rises to ~355 |
| 09:52 start | rises to ~100 °C | 1 | 43 → 100 within 3 min | 65 | 0x18 | drops to ~0 | drops to −76 |
| 10:04 defrost | drops briefly | 1 | 0 | 65 | 0x108 / 0x118 | spike 674 | spike 270 |
| 10:09–10:27 | ~90 °C | 1 | 76 → 83, then down to 44 | 65 | 0x18 | −18 … +3 | −7 … 33 |

0x0640 (ZWISCHENEINSPRITZUNG_ISTTEMP_VND) stayed constant (heater 34, boiler/manager 16) – no information so far.

**Interim result:** compressor running is confirmed by four independent signals (hot gas, bit 0x0200 of 0x0061,
0x0176 = 65, bit 0x10 of 0x07A8). 0x063D is the best candidate for the compressor's modulation and should replace
0x063F as the "speed" metric. 0x063F and 0x033D both behave like controller values (rising while standing still),
with different scales – what 0x063F is stays **open**. More cycles are needed to confirm, especially whether
0x063D ever exceeds 100.

**Evaluation of three cycles (09:14–12:53, −5 … 0 °C, three defrosts at 10:04, 10:41, 11:59):**

| Cycle | Phase (bit 0x0200 of 0x0061) | Hot gas | 0x063D | Notes |
|---|---|---|---|---|
| stop | 09:18–09:51 (34 min) | 94 → 53 °C | 0 | 0x063F before start 355, 0x033D 233 |
| run | 09:52–10:44 (53 min) | up to 105 °C | 0, 43 … 100 | defrosts 10:04 and 10:41 |
| stop | 10:45–11:18 (34 min) | 73 → 45 °C | 0 | 0x063F before start 277, 0x033D 194 |
| run | 11:19–12:24 (66 min) | up to 86 °C | 0, 28 … 76 | defrost 11:59, outdoor ~0 °C |
| stop | 12:25– | 75 → 46 °C | 0 | |

- **0x063D is the compressor modulation in percent** (values 0 or 28–100, maximum exactly 100).
- **Bit 0x100 of 0x07A8 is the defrost**: set at all three defrosts, to the minute together with the defrost
  counter 0x0806.
- **Compressor running**: bit 0x10 of 0x07A8 and 0x063D > 0 match exactly and both drop during defrosts. Bit 0x0200
  of 0x0061 and 0x0176 = 65 mark the whole heating cycle including its defrosts.
- 0x063F and 0x033D have no fixed start threshold – their meaning stays **open**; they are not needed for the
  metrics.
- 0x0640 stayed constant (heater 34, boiler/manager 16).

**Consequences** (implemented in the PR "feat!: compressor metrics from verified registers"):

- export 0x063D as `stiebel_compressor_modulation_percent` and drop `stiebel_compressor_speed_rpm` (0x063E is
  always 0, 0x063F is not a speed);
- new `stiebel_compressor_running` from bit 0x10 of 0x07A8 and `stiebel_heater_defrost_active` from bit 0x100;
- `stiebel_heater_defrost_phase` becomes `stiebel_heating_cycle_active` (bit 0x0200 of 0x0061);
- the alert `HeatPumpCompressorStopped` is based on these metrics (e.g. no heating cycle for longer than the
  usual standstill).
