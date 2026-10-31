# SLZB-06 Prometheus Exporter

A Prometheus exporter for SLZB-06 Zigbee 3.0 PoE Ethernet USB Adapters.

It also reads the SLZB-OS JSON endpoints (`/ha_info` and `/ha_sensors`) when
the device serves them, which covers the multi-radio SLZB-Ultima range and
SLZB-06 devices on core firmware 2.x and later. Support is detected on every
collection cycle, so there is nothing to configure per model. See
[API.md](API.md) for the endpoints themselves.

## Supported devices

- SLZB-06 family: the legacy `/api` endpoint.
- SLZB-OS `/ha_info` and `/ha_sensors` API: validated on a real SLZB-Ultima3
  running v3.4.5.dev0.
- SLZB-06 on core firmware 2.x: expected to work through the same SLZB-OS API,
  not validated on hardware.

**Image**: `ghcr.io/d0ugal/slzb-exporter:v2.16.60`

## Metrics

### Device Information

- `slzb_device_connected` - Device connection status (1=connected, 0=disconnected)
- `slzb_device_temperature_celsius` - Device temperature in Celsius
- `slzb_device_uptime_seconds` - Device uptime in seconds
- `slzb_device_heap_free_kb` - Free heap memory in kilobytes
- `slzb_device_heap_size_kb` - Total heap memory in kilobytes
- `slzb_device_heap_ratio` - Heap usage ratio as percentage
- `slzb_device_ethernet_connected` - Ethernet connection status
- `slzb_device_wifi_connected` - WiFi connection status
- `slzb_device_operational_mode` - Device operational mode
- `slzb_device_reachable` - Device reachability status

### HTTP Request Metrics

- `slzb_http_requests_total` - Total number of HTTP requests
- `slzb_http_errors_total` - Total number of HTTP errors
- `slzb_last_collection_timestamp` - Unix timestamp of last successful collection
- `slzb_collection_errors_total` - Total number of collection errors

The `action` label of the request, latency and error metrics is `0`, `4` or
`firmware` for the legacy API and `ha_info` or `ha_sensors` for the SLZB-OS
endpoints. A device that does not serve `/ha_info` is not an error: the
request is counted in `slzb_http_requests_total`, never in
`slzb_http_errors_total`. A device answering HTTP 401 is counted with
`error_type="auth_error"` (see [Authentication](#authentication)).

### SLZB-OS / multi-radio (SLZB-Ultima, SLZB-06 core 2.x+)

These gauges come from `/ha_info` and `/ha_sensors`. Every one of them carries
the `device` label; the table lists the extra labels. A gauge only exists when
the firmware sent its source key, so a single-radio SLZB-06 exposes fewer
series than an SLZB-Ultima. When a key disappears from the payload, its series
is deleted on the next cycle, and when the device stops serving `/ha_info`,
all of them are.

Sizes are sent by the firmware in kilobytes and exported in bytes (KB x 1024).

| Metric | Extra labels | Source | Meaning |
| --- | --- | --- | --- |
| `slzb_device_info` | `model`, `hostname`, `sw_version`, `hw_version`, `mac` | `/ha_info` | Device identity, always 1 |
| `slzb_radio_info` | `radio`, `chip`, `type`, `channel`, `firmware_version` | `/ha_info` `radios[]` | One series per radio chip, always 1 |
| `slzb_device_ram_size_bytes` | | `/ha_info` `ram_total` | Total RAM |
| `slzb_device_psram_size_bytes` | | `/ha_info` `psram_total` | Total PSRAM |
| `slzb_device_fs_size_bytes` | | `/ha_info` `fs_total` | Filesystem size |
| `slzb_device_addon_present` | `addon` | `/ha_info` `addons` | Hardware add-on fitted (1) or not (0); `addon` is `lte`, `poe`, `zwave` or `mic` |
| `slzb_device_ble_enabled` | | `/ha_info` `ble.ble_enabled` | Bluetooth LE enabled in the configuration |
| `slzb_device_ble_proxy_enabled` | | `/ha_info` `ble.proxy_enabled` | Bluetooth LE proxy enabled in the configuration |
| `slzb_radio_temperature_celsius` | `sensor` | `/ha_sensors` `zb_temp`, `zb_temp2` | Radio temperature; `sensor` is `zb_temp` or `zb_temp2` |
| `slzb_socket_client_uptime_seconds` | `socket` | `/ha_sensors` `socket_uptime`, `socket2_uptime`, `socket3_uptime` | TCP socket client uptime, 0 when no client; `socket` is `1`, `2` or `3` |
| `slzb_socket_client_connected` | `socket` | same as above | 1 when the socket uptime is above 0 |
| `slzb_otbr_uptime_seconds` | | `/ha_sensors` `otbr_uptime` | OpenThread border router uptime, 0 when not running |
| `slzb_device_network_connected` | `interface` | `/ha_sensors` `ethernet`, `wifi_connected`, `vpn_status` | Link state; `interface` is `ethernet`, `wifi` or `vpn` |
| `slzb_device_vpn_enabled` | | `/ha_sensors` `vpn_enabled` | VPN enabled |
| `slzb_device_ram_used_bytes` | | `/ha_sensors` `ram_usage` | Used RAM |
| `slzb_device_psram_used_bytes` | | `/ha_sensors` `psram_usage` | Used PSRAM |
| `slzb_device_fs_used_bytes` | | `/ha_sensors` `fs_used` | Used filesystem space |
| `slzb_device_ram_largest_free_block_bytes` | | `/ha_sensors` `ram_largest_free_block` | Largest contiguous free RAM block (fragmentation signal); only on firmware that sends it, such as SLZB-Ultima3 v3.4.5 |
| `slzb_device_psram_largest_free_block_bytes` | | `/ha_sensors` `psram_largest_free_block` | Largest contiguous free PSRAM block (fragmentation signal); same availability |
| `slzb_device_ble_state` | | `/ha_sensors` `ble.state` | Bluetooth LE stack state, see the mapping below |
| `slzb_device_ble_proxy_connected` | | `/ha_sensors` `ble.proxy_connected` | Bluetooth LE proxy client connected |
| `slzb_device_lte_detected` | | `/ha_sensors` `lte_detect` | LTE modem detected |
| `slzb_device_lte_state` | | `/ha_sensors` `lte_state` | LTE connection state, see the mapping below |

Label values mirror what the firmware sends: `model` and `sw_version` are
not rewritten, and `hw_version` is the raw integer text (for example `104`,
not `1.04`). `radio` is the chip index (`0`, `1`, `2`) and `chip` the radio
hardware name (for example `EFR32MG24`).

Which radio chip `zb_temp` and `zb_temp2` belong to is not documented by
SMLIGHT, so the `sensor` label keeps the firmware key names. The single-radio
SLZB-06 also sends `zb_temp2`.

Some of these overlap existing metrics on purpose, so that the SLZB-OS metric
set is the same on every device that serves it:

- socket `1` overlaps `slzb_socket_uptime_seconds` and `slzb_socket_connected`;
- `interface="ethernet"` and `interface="wifi"` overlap
  `slzb_device_ethernet_connected` and `slzb_device_wifi_connected`;
- the RAM gauges overlap the `slzb_device_heap_*` metrics.

Two existing metrics are fed from `/ha_sensors` only when the legacy
`/api?action=0` endpoint gave no data during the cycle, so they never have two
competing sources: `slzb_device_temperature_celsius` (from `esp32_temp`) and
`slzb_device_uptime_seconds` (from `uptime`). In the same situation,
`slzb_firmware_current_version` takes its `version` label from the `/ha_info`
`sw_version`.

#### Enum mappings

These follow `const.py` of [pysmlight](https://github.com/smlight-tech/pysmlight),
SMLIGHT's reference client. A code missing from a table is exported as its
decimal text (for example `type="3"`), so a value added by newer firmware
stays distinct instead of collapsing into `unknown`.

`slzb_radio_info` `type` label (`zb_type`):

| Code | Label value |
| --- | --- |
| -1 | `unknown` |
| 0 | `coordinator` |
| 1 | `router` |
| 2 | `thread` |
| 4 | `zhub` |
| 5 | `Zwave-EU` |
| 6 | `Zwave-US` |
| 7 | `Zwave-ANZ` |
| 8 | `otbr` |

`slzb_radio_info` `channel` label (`zb_channel`, the radio firmware release
channel):

| Code | Label value |
| --- | --- |
| -1 | `unknown` |
| 0 | `stable` |
| 1 | `dev` |
| 2 | `custom` |

`slzb_device_ble_state` value:

| Value | State |
| --- | --- |
| 0 | idle |
| 1 | starting |
| 2 | started |
| 3 | error |
| 4 | stopping |
| 5 | stopped |

`slzb_device_lte_state` value:

| Value | State |
| --- | --- |
| 0 | idle |
| 1 | starting |
| 2 | connecting |
| 3 | connected |
| 4 | error |
| 5 | no_signal |

#### Radio normalisation

The exporter applies the same rules as pysmlight before exporting radios:

- a radio reporting `zb_version` -1 has no known firmware: its
  `firmware_version` label is empty and its `channel` is forced to `custom`;
- an SLZB-Ultima without the `zwave` add-on reports an empty Z-Wave slot as a
  third radio, which is dropped;
- firmware that sends no `radios` array gets radio `0` built from its
  top-level `zb_*` fields.

### Endpoints

- `GET /`: HTML dashboard with device status and metrics information
- `GET /metrics`: Prometheus metrics endpoint
- `GET /health`: Health check endpoint

## Quick Start

### Docker Compose

```yaml
version: '3.8'
services:
  slzb-exporter:
    image: ghcr.io/d0ugal/slzb-exporter:v2.16.60
    ports:
      - "9110:9110"
    environment:
      - SLZB_EXPORTER_SLZB_API_URL=http://your-slzb-device.local
      # Only when the device web UI is protected by a login:
      # - SLZB_EXPORTER_SLZB_USERNAME=admin
      # - SLZB_EXPORTER_SLZB_PASSWORD=change-me
    restart: unless-stopped
```

1. Update the SLZB device URL in the environment variables
2. Run: `docker-compose up -d`
3. Access metrics: `curl http://localhost:9110/metrics`

## Configuration

The exporter is configured via environment variables:

### Environment Variables

- `SLZB_EXPORTER_SLZB_API_URL` - Base URL of the SLZB-06 device (default: `http://slzb-device.local`)
- `SLZB_EXPORTER_SLZB_USERNAME` - Username of the device web UI, for HTTP Basic auth (default: empty)
- `SLZB_EXPORTER_SLZB_PASSWORD` - Password of the device web UI, for HTTP Basic auth (default: empty)
- `SLZB_EXPORTER_METRICS_DEFAULT_INTERVAL` - Collection interval (default: 10s)
- `SLZB_EXPORTER_SERVER_HOST` - Server host (default: 0.0.0.0)
- `SLZB_EXPORTER_SERVER_PORT` - Server port (default: 9110)
- `SLZB_EXPORTER_LOG_LEVEL` - Log level: debug, info, warn, error (default: info)
- `SLZB_EXPORTER_LOG_FORMAT` - Log format: json, text (default: json)

### Authentication

Set `SLZB_EXPORTER_SLZB_USERNAME` and `SLZB_EXPORTER_SLZB_PASSWORD` when the
device web UI is protected by a password. A device with a password answers
HTTP 401 on every endpoint, the legacy `/api` as well as `/ha_info` and
`/ha_sensors`, so without credentials the exporter collects nothing.

- Auth is only used when both variables are set; one of them alone is
  ignored.
- Every request to the device then carries an `Authorization: Basic` header.
  It is only sent to the host of `SLZB_EXPORTER_SLZB_API_URL`, never to
  another host the device would redirect to.
- The password is never logged, traced or included in error messages.
- A 401 is counted in `slzb_http_errors_total` with `error_type="auth_error"`
  and logged once per cycle at warn level, with a hint naming the two
  variables.

Prefer passing the password from a secret store (a Docker or Kubernetes
secret) over writing it in a compose file.

### Example Configuration

```bash
# Basic configuration
export SLZB_EXPORTER_SLZB_API_URL="http://192.168.1.100"
export SLZB_EXPORTER_METRICS_DEFAULT_INTERVAL="60s"

# Device web UI credentials, only when it is password protected
export SLZB_EXPORTER_SLZB_USERNAME="admin"
export SLZB_EXPORTER_SLZB_PASSWORD="change-me"

# Advanced configuration
export SLZB_EXPORTER_SERVER_HOST="0.0.0.0"
export SLZB_EXPORTER_SERVER_PORT="9110"
export SLZB_EXPORTER_LOG_LEVEL="info"
export SLZB_EXPORTER_LOG_FORMAT="json"
```

## Deployment

### Docker Compose (Environment Variables)

```yaml
version: '3.8'
services:
  slzb-exporter:
    image: ghcr.io/d0ugal/slzb-exporter:v2.16.60
    ports:
      - "9110:9110"
    environment:
      - SLZB_EXPORTER_SLZB_API_URL=http://192.168.1.100
      - SLZB_EXPORTER_METRICS_DEFAULT_INTERVAL=60s
    restart: unless-stopped
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: slzb-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: slzb-exporter
  template:
    metadata:
      labels:
        app: slzb-exporter
    spec:
      containers:
      - name: slzb-exporter
        image: ghcr.io/d0ugal/slzb-exporter:v2.16.60
        ports:
        - containerPort: 9110
        env:
        - name: SLZB_EXPORTER_SLZB_API_URL
          value: "http://192.168.1.100"
        - name: SLZB_EXPORTER_METRICS_DEFAULT_INTERVAL
          value: "60s"
```

For a password protected device, add the credentials from a Secret:

```yaml
        - name: SLZB_EXPORTER_SLZB_USERNAME
          valueFrom:
            secretKeyRef:
              name: slzb-exporter
              key: username
        - name: SLZB_EXPORTER_SLZB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: slzb-exporter
              key: password
```

## Prometheus Integration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'slzb-exporter'
    static_configs:
      - targets: ['slzb-exporter:9110']
    scrape_interval: 30s
```

## Example PromQL Queries

### Device Health

```promql
# Device reachability
slzb_device_reachable

# Device temperature
slzb_device_temperature_celsius

# Heap memory usage
slzb_device_heap_ratio
```

### SLZB-OS Devices

```promql
# Radio temperatures, one series per firmware sensor key
slzb_radio_temperature_celsius

# RAM usage ratio
slzb_device_ram_used_bytes / slzb_device_ram_size_bytes

# RAM fragmentation: share of the free RAM that is not in the largest block
# (close to 1 means plenty free in total but no large allocation possible)
1 - slzb_device_ram_largest_free_block_bytes
  / (slzb_device_ram_size_bytes - slzb_device_ram_used_bytes)

# Radios and their firmware, as a table
slzb_radio_info

# Socket clients currently connected
sum by (device) (slzb_socket_client_connected)

# Devices whose LTE modem is not connected (3 = connected)
slzb_device_lte_state != 3

# Authentication failures over the last hour
increase(slzb_http_errors_total{error_type="auth_error"}[1h])
```

## Development

### Building

```bash
make build
```

### Testing

```bash
make test
```

### Linting

```bash
make lint
```

## License

This project is licensed under the MIT License.
