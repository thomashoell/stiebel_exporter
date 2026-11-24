# Roadmap

Ideas for later – nothing here is implemented yet. Each entry lists the open questions that have to be answered
before work starts.

## Web UI with the current values

A small page served by the exporter that shows the current values, like a compact version of the Grafana
dashboard (temperatures, set points, compressor state, pressures, hot water charging, defrosts).

- Served on the same listener as `/metrics`, embedded into the binary (`embed` + `html/template`), no frontend
  build chain – keeps the deployment on daedalus a single binary.
- Reads the values the exporter already holds; no extra CAN requests.
- Later also serves the documentation (`docs/*.md`, e.g. [heat-pump-registers.md](heat-pump-registers.md)),
  rendered as HTML.

## Changing set points (API, optionally the web UI)

Use case: heat the hot water tank when there is enough PV surplus, so it doesn't have to happen at night.

- Writing is not supported yet; the protocol pieces exist (`stiebel.EncodeValue`, `EncodeReceiver`), a write
  telegram to the boiler/WPM (`0x180`) is missing.
- Candidates: `SPEICHERSOLLTEMP` (0x0003) to trigger hot water charging; possibly `PROGRAMMSCHALTER` (0x0112).
- Open questions:
  - Which values does the WPM persist (EEPROM), and how often may they be written? Never write in a fast loop.
  - Safe fallback: a temporary set point must be restored even if the exporter crashes or loses the CAN bus
    (time limit in the exporter, restore on start, or a fixed fallback value).
  - Allowed ranges per set point, validated by the API.
  - Alternative without CAN writes: the WPM's SG Ready input (potential-free contact), if this device has one.
- Authentication: OIDC via the homelab Keycloak (the exporter has been read-only so far; reads stay
  anonymous, writes require a token/login).

## Integration with PV surplus and the wallbox

Charging the car has priority over heating the hot water tank.

- The decision who gets the surplus does not belong into the exporter. The exporter only offers a narrow
  interface (e.g. "hot water boost on/off", set point with time limit); an orchestrator that sees PV
  (sunspec-exporter), wallbox and heat pump decides.
- Orchestrator candidates:
  - evcc: load points with priorities, heat pump integration (SG Ready or custom). Tried before – it had problems
    with the wallbox's phase switching; check whether that has been fixed before ruling it out.
  - Home Assistant: automations with a REST/MQTT integration of the exporter's API.
