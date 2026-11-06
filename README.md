# stiebel_exporter - Prometheus exporter for Stiebel Eltron heat pumps

[![CI](https://github.com/thomashoell/stiebel_exporter/actions/workflows/release.yml/badge.svg?branch=main)](https://github.com/thomashoell/stiebel_exporter/actions/workflows/release.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/thomashoell/stiebel_exporter/badge)](https://scorecard.dev/viewer/?uri=github.com/thomashoell/stiebel_exporter)

*stiebel_exporter* reads values from a Stiebel Eltron heat pump over its CAN bus (Elster protocol) and exports
them as Prometheus metrics. It acts like ComfortSoft (CAN ID `0x680`): every interval it requests a fixed set
of values from the boiler (`0x180`) and the heat module (`0x500`) and exports the responses.

The project was called *wlp_exporter* before; the metric names did not change. When upgrading a host, stop and
disable the old `wlp_exporter` service before enabling `stiebel_exporter`.

## Metrics

| Metric | Labels | Elster indexes |
|---|---|---|
| `stiebel_boiler_flow_temperature_celsius` | | `WPVORLAUFIST` (0x01D6) |
| `stiebel_boiler_return_temperature_celsius` | | `RUECKLAUFISTTEMP` (0x0016) |
| `stiebel_boiler_tank_temperature_celsius` | | `SPEICHERISTTEMP` (0x000E) |
| `stiebel_outdoor_temperature_celsius` | | `AUSSENTEMP` (0x000C) |
| `stiebel_heater_hot_gas_temperature_celsius` | | `HEISSGAS_TEMP` (0x0265) |
| `stiebel_compressor_speed_rpm` | `unit` | `ISTDREHZAHL_VERDICHTER_1/2` (0x063E/0x063F) |
| `stiebel_electrical_power_consumption_watt_hours` | `time_frame`, `magnitude`, `component` | `EL_AUFNAHMELEISTUNG_*` (0x091A–0x0921) |
| `stiebel_heat_produced_watt_hours` | `time_frame`, `magnitude`, `component`, `heat_generator` | `WAERMEERTRAG_*` (0x0922–0x0931) |

The heat pump reports energy in parts (Wh and kWh of the day, kWh and MWh of the total). Each part is
exported in Wh with its `magnitude` (`WH`, `kWH`, `MWH`); the value of a `time_frame` is the sum of its parts.

The exporter itself exposes `stiebel_exporter_build_info`, `stiebel_exporter_responses_total{unit}`,
`stiebel_exporter_last_response_timestamp_seconds{unit}` (alert on stale data with this),
`stiebel_exporter_publish_errors_total` and the Go/process metrics.

### Experimental metrics

With `-test-metrics`, the exporter additionally asks the boiler, the heat module and the manager (`0x480`) for
about 35 more Elster indexes (set points, room, evaporator and condenser temperatures, refrigerant pressures,
flow, fan speed, defrosting, run times, status words; see `metrics/testmetrics.go`) and exports every answer as

```
test_stiebel_register_value{unit="manager",index="0x0011",name="RAUMISTTEMP"} 21.5
```

Indexes a unit doesn't know never show up. Names and scaling come from the can_progs table and are not verified:
compare the values with the heat pump's display, then promote the useful ones to proper `stiebel_*` metrics.
The test values are polled once per `-test-interval` (each request takes ~50 ms of bus time).

## Usage

```
stiebel_exporter [-device can0] [-listen :9000] [-interval 10s] [-log-level info]
```

| Flag | Default | Description |
|---|---|---|
| `-device` | `can0` | CAN interface connected to the heat pump |
| `-listen` | `:9000` | address of the metrics endpoint (`/metrics`) |
| `-interval` | `10s` | interval between two polls of the heat pump |
| `-log-level` | `info` | `debug` logs every frame that is not exported |
| `-test-metrics` | `false` | also request experimental values, see below |
| `-test-interval` | `1m` | interval between two polls of the experimental values |
| `-version` | | print the version and exit |

The CAN interface has to be up before the exporter starts. The exporter exits with a non-zero status if the
interface fails, so systemd can restart it.

## Installation

The [releases](https://github.com/thomashoell/stiebel_exporter/releases) contain Linux binaries for `amd64`,
`arm64` and `armv7` (Raspberry Pi with a 32 bit OS; with a 64 bit kernel, `uname -m` = `aarch64`, the `arm64`
binary runs as well). `checksums.txt` is signed with Sigstore and the archives
have build provenance attestations:

```bash
version=1.0.0
curl -fsSLO "https://github.com/thomashoell/stiebel_exporter/releases/download/${version}/stiebel_exporter_${version}_linux_armv7.tar.gz"
gh attestation verify "stiebel_exporter_${version}_linux_armv7.tar.gz" --repo thomashoell/stiebel_exporter
tar xzf "stiebel_exporter_${version}_linux_armv7.tar.gz"
sudo install -m 0755 stiebel_exporter /usr/local/bin/
sudo install -m 0644 stiebel_exporter.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now stiebel_exporter
```

## Development

Requires Go 1.26 or later. CI runs [golangci-lint](https://golangci-lint.run) (including `gofmt`/`goimports`)
on every pull request. To catch issues before pushing, install [pre-commit](https://pre-commit.com) and enable
the hooks once per clone:

```bash
sudo dnf install pre-commit   # or: pipx install pre-commit
make hooks                    # runs `pre-commit install`
```

`make fmt` formats the code, `make lint` runs the linters, `make test` runs the tests, `make build-daedalus`
builds the binary for the Raspberry Pi.

To try the exporter without a heat pump, use a virtual CAN interface and answer its requests with `cansend`
(can-utils):

```bash
sudo modprobe vcan && sudo ip link add dev vcan0 type vcan && sudo ip link set up vcan0
./stiebel_exporter -device vcan0 -log-level debug &
cansend vcan0 180#D2000C007B    # boiler: outdoor temperature 12.3 °C
curl -s localhost:9000/metrics | grep outdoor
```

### Releases

Pull requests are squash-merged; the PR title has to be a [Conventional Commit](https://www.conventionalcommits.org/)
(checked by CI). On every push to `main`, the release workflow runs CI, lets
[semantic-release](https://github.com/semantic-release/semantic-release) derive the next version from the
commits (`feat` → minor, `fix`/`perf`/`build(deps)` → patch, `!` → major), updates `CHANGELOG.md`, creates the
tag and the GitHub release, and GoReleaser attaches the binaries, checksums, signatures and SBOMs.

## Credits

The Elster index definitions are taken from [can_progs](http://juerg5524.ch/list_data.php) by Jürg Müller.
