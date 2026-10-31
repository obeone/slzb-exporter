# SLZB-06 API Documentation

This document describes the SLZB-06 device API that this exporter consumes. This information is useful for developers who want to understand how the exporter works or contribute to its development.

## API Discovery

Based on exploration of the SLZB-06 device, the following API endpoints have been discovered:

## Available API Endpoints

Based on the firmware source code at [smlight-dev/slzb-06-firmware](https://github.com/smlight-dev/slzb-06-firmware/blob/main/src/web.cpp#L151), the API uses the following action enumeration:

```cpp
enum API_ACTION_t : uint8_t { 
    API_GET_PAGE,           // 0
    API_GET_PARAM,          // 1
    API_STARTWIFISCAN,      // 2
    API_WIFISCANSTATUS,     // 3
    API_GET_FILELIST,       // 4
    API_GET_FILE,           // 5
    API_SEND_HEX,           // 6
    API_WIFICONNECTSTAT,    // 7
    API_CMD,                // 8
    API_GET_LOG             // 9
};
```

All endpoints use the pattern: `GET /api?action=<action>&page=<page>`

### Action 0 (API_GET_PAGE): HTML Template

- **URL**: `/api?action=0&page=0`
- **Response**: Gzipped HTML template with data-replace placeholders
- **Content**: Device status page template with placeholders like `data-replace="operationalMode"`, `data-replace="connectedEther"`, etc.

### Action 1 (API_GET_PARAM): Get Parameters

- **URL**: `/api?action=1&param=<param_name>`
- **Response**: Text response with parameter value
- **Parameters**:
  - `refreshLogs` - Returns refresh logs setting
  - `coordMode` - Returns coordinator mode (1=wifi setup, otherwise coordinator_mode setting)

### Action 2 (API_STARTWIFISCAN): Start WiFi Scan

- **URL**: `/api?action=2&page=0`
- **Response**: "ok" if successful
- **Note**: Enables WiFi in STA mode if it was off

### Action 3 (API_WIFISCANSTATUS): WiFi Scan Results

- **URL**: `/api?action=3&page=0`
- **Response**: JSON with WiFi scan data
- **Example**:

```json
{
  "scanDone": true,
  "wifi": [
    {
      "ssid": "ctu",
      "rssi": -49,
      "channel": 6,
      "secure": 3
    }
  ]
}
```

### Action 4 (API_GET_FILELIST): File List

- **URL**: `/api?action=4&page=0`
- **Response**: JSON with configuration files
- **Example**:

```json
{
  "files": [
    {
      "filename": "configEther.json",
      "size": 36
    },
    {
      "filename": "configGeneral.json", 
      "size": 141
    }
  ]
}
```

### Action 5 (API_GET_FILE): Get File Content

- **URL**: `/api?action=5&filename=<filename>`
- **Response**: File content as text
- **Note**: Files are read from `/config/` directory

### Action 6 (API_SEND_HEX): Send Hex Data

- **URL**: `/api?action=6&hex=<hex_data>&size=<size>`
- **Response**: "ok" if successful
- **Note**: Sends hex data to Serial2 (Zigbee communication)

### Action 7 (API_WIFICONNECTSTAT): WiFi Connection Status

- **URL**: `/api?action=7&page=0`
- **Response**: JSON with WiFi connection status
- **Example**:

```json
{
  "connected": false
}
```

- **Note**: Returns `{"connected": true, "ip": "192.168.1.100"}` if connected

### Action 8 (API_CMD): Execute Commands

- **URL**: `/api?action=8&cmd=<command_id>`
- **Response**: "ok" if successful
- **Commands**:
  - `0` - CMD_ZB_ROUTER_RECON: Router reconnect
  - `1` - CMD_ZB_RST: Zigbee restart
  - `2` - CMD_ZB_BSL: Zigbee enable BSL
  - `3` - CMD_ESP_RES: ESP32 restart
  - `4` - CMD_ADAP_LAN: Adapter mode LAN
  - `5` - CMD_ADAP_USB: Adapter mode USB
  - `6` - CMD_LEDY_TOG: LED Yellow toggle
  - `7` - CMD_LEDB_TOG: LED Blue toggle
  - `8` - CMD_CLEAR_LOG: Clear log

### Action 9 (API_GET_LOG): Get Log

- **URL**: `/api?action=9&page=0`
- **Response**: Log content as text
- **Note**: Returns the device log output

## Data-Replace Placeholders Found

From the HTML template, these are the metrics available:

### Device Status

- `operationalMode` - Device operational mode
- `connectedEther` - Ethernet connected status
- `connectedSocketStatus` - Socket client connected status
- `wifiEnabled` - WiFi Client enabled status
- `wifiConnected` - WiFi Client connection status
- `wifiModeAP` - WiFi Access Point enabled status
- `wifiModeAPStatus` - WiFi Access Point status
- `uptime` - Device uptime
- `connectedSocket` - Socket uptime

### Device Information

- `hwRev` - Hardware revision/model
- `VERSION` - ESP32 Firmware version
- `espModel` - ESP32 version
- `deviceTemp` - ESP32 temperature (°C)
- `espCores` - Number of ESP32 cores
- `espFreq` - ESP32 frequency (MHz)
- `espFlashSize` - ESP32 flash size (MB)
- `espFlashType` - ESP32 flash type
- `espHeapFree` - ESP32 free heap (KB)
- `espHeapSize` - ESP32 total heap size (KB)

### Ethernet

- `ethConnection` - Connection status
- `ethDhcp` - DHCP status
- `ethIp` - IP Address
- `etchMask` - Subnet Mask
- `ethGate` - Default Gateway
- `ethSpd` - Connection speed
- `ethMac` - MAC address

### WiFi

- `wifiMode` - WiFi mode
- `wifiSsid` - SSID
- `wifiMac` - MAC Address
- `wifiIp` - IP Address
- `wifiSubnet` - Subnet Mask
- `wifiGate` - Default Gateway
- `wifiRssi` - RSSI
- `wifiDhcp` - DHCP status

## SLZB-OS Home Assistant Endpoints

SMLIGHT devices running SLZB-OS (the SLZB-Ultima range, and SLZB-06 devices
on core firmware 2.x and later) serve two JSON endpoints that the Home
Assistant integration reads: `GET /ha_info` and `GET /ha_sensors`.

### Source

SMLIGHT does not publish a specification for these endpoints. Everything below
comes from [pysmlight](https://github.com/smlight-tech/pysmlight), SMLIGHT's
own Python client used by Home Assistant: `pysmlight/web.py` for the requests,
`pysmlight/models.py` for the fields, `pysmlight/const.py` for the enums, and
the `tests/fixtures/slzb-ultima-*.json` and `slzb-06-*.json` files for sample
payloads. The test fixtures of this exporter, in
`internal/slzbos/testdata/`, are rewritten from those samples.

### Authentication

When the device web UI is password protected, every endpoint (`/ha_info`,
`/ha_sensors`, `/api`, `/api2`) answers HTTP 401 with the body
`wrong login or password` until the request carries an
`Authorization: Basic` header, which is what pysmlight sends. The exporter adds
that header to every device request when both `SLZB_EXPORTER_SLZB_USERNAME`
and `SLZB_EXPORTER_SLZB_PASSWORD` are set, and only for the host of
`SLZB_EXPORTER_SLZB_API_URL`.

### Detecting Support

The exporter requests `/ha_info` on every collection cycle, without caching
the result, then `/ha_sensors` only when `/ha_info` parsed. Neither is
requested in a cycle where the legacy `/api?action=0` request failed at the
transport level (timeout, connection refused), so a dead device does not cost
two more client timeouts.

An endpoint is **unsupported** when it answers:

- HTTP 404, or
- a body that is exactly `URL NOT FOUND` (surrounding whitespace ignored),
  whatever the status code: SMLIGHT firmware may send it with a 200, which is
  why pysmlight checks the body and not the status, or
- HTTP 200 with a body that is not a JSON object holding the expected
  top-level key (`Info` for `/ha_info`, `Sensors` for `/ha_sensors`), or
  where that key is `null`. An HTML page falls in this case.

Unsupported is the expected answer of legacy firmware, not an error: the
request is counted in `slzb_http_requests_total` and observed in
`slzb_api_response_time_seconds` (with `action="ha_info"` or
`action="ha_sensors"`), logged at debug level, and never counted in
`slzb_http_errors_total`. An unsupported `/ha_info` removes every SLZB-OS
series of the device; an unsupported `/ha_sensors` (after `/ha_info` parsed)
removes only the series sourced from it.

Anything else is a **failure**, counted in `slzb_http_errors_total` under the
same `action` labels, with the last exported values left in place:

| Answer | `error_type` |
| --- | --- |
| No response (timeout, connection refused) | `request_error` |
| HTTP 401 | `auth_error` |
| Other non-200 status (for example 500) | `http_error` |
| `Info` or `Sensors` object present but not decodable | `json_error` |
| Body could not be read | `read_error` |

A field of the wrong type, such as a numeric field holding a non-numeric
string, makes the whole object undecodable. Response bodies are read up to
1 MiB.

### GET /ha_info

- **URL**: `/ha_info`
- **Response**: `{"Info": {...}}`, device identity and capabilities

Numbers may be sent either as JSON numbers or as numeric strings; the
exporter accepts both. Sizes are in kilobytes.

| Field | Type | Unit | Meaning | Exported as |
| --- | --- | --- | --- | --- |
| `model` | string | | Device model, for example `SLZB-Ultima3`, `SLZB-06P10` | `slzb_device_info{model}` |
| `hostname` | string | | Device hostname | `slzb_device_info{hostname}` |
| `sw_version` | string | | Core (ESP32) firmware version, for example `v3.2.6.dev2` | `slzb_device_info{sw_version}`, firmware version fallback |
| `hw_version` | number | | Hardware revision as an integer, for example `104` (pysmlight displays it as `1.04`) | `slzb_device_info{hw_version}` |
| `MAC` | string | | Device MAC address | `slzb_device_info{mac}` |
| `fw_channel` | string | | Device-level firmware channel (`dev` or `release`) | not exported |
| `sel_fw_channel` | string | | Firmware channel selected for updates | not exported |
| `legacy_api` | number | | Legacy API flag; meaning unconfirmed | not exported |
| `ram_total` | number | KB | Total RAM | `slzb_device_ram_size_bytes` |
| `psram_total` | number | KB | Total PSRAM | `slzb_device_psram_size_bytes` |
| `fs_total` | number | KB | Filesystem size | `slzb_device_fs_size_bytes` |
| `radios` | array | | One object per radio chip, see below | `slzb_radio_info` |
| `addons` | object | | Optional hardware modules as booleans: `lte`, `poe`, `zwave`, `mic` | `slzb_device_addon_present{addon}` for boolean members only |
| `addons.addon_type` | number | | Integer member seen on SLZB-Ultima3 v3.4.5.dev0; meaning undocumented | not exported (non-boolean addon members are skipped) |
| `ble.ble_enabled` | boolean | | Bluetooth LE enabled in the configuration | `slzb_device_ble_enabled` |
| `ble.proxy_enabled` | boolean | | Bluetooth LE proxy enabled in the configuration | `slzb_device_ble_proxy_enabled` |
| `zb_hw`, `zb_version`, `zb_type`, `zb_channel`, `zb_ram_size`, `zb_flash_size` | as in `radios[]` | | Radio 0 duplicated at the top level, the only radio fields on firmware without `radios` | radio `0` when `radios` is absent |
| `coord_mode`, `wifi_mode` | number | | Coordinator and WiFi mode enums | not exported |
| `device_ip` | string | | Device IP address | not exported |
| `u_device`, `mr_device`, `ult_device` | boolean | | Device family flags (multi-radio, Ultima) | not exported |

Fields of each `radios[]` entry:

| Field | Type | Unit | Meaning | Exported as |
| --- | --- | --- | --- | --- |
| `chip_index` | number | | Radio index, from 0 | `slzb_radio_info{radio}` |
| `zb_hw` | string | | Radio chip, for example `EFR32MG24`, `CC2674P10`, `EFR32ZG23` | `slzb_radio_info{chip}` |
| `zb_version` | number | | Radio firmware version, often a date such as `20251218`; -1 when unknown | `slzb_radio_info{firmware_version}` |
| `zb_type` | number | | Radio role, see the enum below | `slzb_radio_info{type}` |
| `zb_channel` | number | | Radio firmware release channel, see the enum below | `slzb_radio_info{channel}` |
| `zb_ram_size` | number | KB | Radio chip RAM | not exported |
| `zb_flash_size` | number | KB | Radio chip flash | not exported |
| `radioModes` | array of booleans | | Modes the chip supports; undocumented | not exported |

`zb_type` values: -1 unknown, 0 coordinator, 1 router, 2 thread, 4 zhub,
5 Zwave-EU, 6 Zwave-US, 7 Zwave-ANZ, 8 otbr.

`zb_channel` values: -1 unknown, 0 stable, 1 dev, 2 custom.

Normalisation, applied by pysmlight and by this exporter:

- `zb_version` -1 means no known firmware: the version is dropped and
  `zb_channel` is forced to 2 (custom);
- on a model whose name contains `Ultima` and whose `addons.zwave` is not
  true, `radios` is truncated to its first 2 entries, as the firmware still
  lists the empty Z-Wave slot;
- when `radios` is absent or empty, radio 0 is built from the top-level
  `zb_*` fields.

pysmlight also rewrites some values for display (model case, `hw_version`
as `1.04`, `sw_version` suffixes). The exporter does not: label values keep
the raw firmware text.

### GET /ha_sensors

- **URL**: `/ha_sensors`
- **Response**: `{"Sensors": {...}}`, live readings

| Field | Type | Unit | Meaning | Exported as |
| --- | --- | --- | --- | --- |
| `esp32_temp` | string or number | °C | ESP32 temperature | `slzb_device_temperature_celsius` when the legacy API gave no data |
| `zb_temp` | string or number | °C | Radio temperature; chip mapping undocumented | `slzb_radio_temperature_celsius{sensor="zb_temp"}` |
| `zb_temp2` | string or number | °C | Second radio temperature; also sent by the single-radio SLZB-06 | `slzb_radio_temperature_celsius{sensor="zb_temp2"}` |
| `uptime` | number | s | Device uptime | `slzb_device_uptime_seconds` when the legacy API gave no data |
| `socket_uptime` | number | s | TCP socket 1 client uptime, 0 when no client | `slzb_socket_client_uptime_seconds{socket="1"}` |
| `socket2_uptime` | number | s | TCP socket 2 client uptime, 0 when no client | `slzb_socket_client_uptime_seconds{socket="2"}` |
| `socket3_uptime` | number | s | TCP socket 3 client uptime, 0 when no client | `slzb_socket_client_uptime_seconds{socket="3"}` |
| `otbr_uptime` | number | s | OpenThread border router uptime, 0 when not running | `slzb_otbr_uptime_seconds` |
| `ram_usage` | number | KB | Used RAM | `slzb_device_ram_used_bytes` |
| `psram_usage` | number | KB | Used PSRAM | `slzb_device_psram_used_bytes` |
| `fs_used` | number | KB | Used filesystem space | `slzb_device_fs_used_bytes` |
| `ram_largest_free_block` | number | KB | Largest contiguous free RAM block; seen on SLZB-Ultima3 v3.4.5.dev0, absent from pysmlight | `slzb_device_ram_largest_free_block_bytes` |
| `psram_largest_free_block` | number | KB | Largest contiguous free PSRAM block; same availability | `slzb_device_psram_largest_free_block_bytes` |
| `ethernet` | boolean | | Ethernet link up | `slzb_device_network_connected{interface="ethernet"}` |
| `wifi_connected` | boolean | | WiFi client connected | `slzb_device_network_connected{interface="wifi"}` |
| `wifi_status` | number | | WiFi mode enum (off, client, AP) | not exported |
| `vpn_status` | boolean | | VPN connected | `slzb_device_network_connected{interface="vpn"}` |
| `vpn_enabled` | boolean | | VPN enabled | `slzb_device_vpn_enabled` |
| `lte_detect` | boolean | | LTE modem detected | `slzb_device_lte_detected` |
| `lte_state` | number | | LTE connection state, see the enum below | `slzb_device_lte_state` |
| `ble.state` | number | | Bluetooth LE stack state, see the enum below | `slzb_device_ble_state` |
| `ble.proxy_connected` | boolean | | Bluetooth LE proxy client connected | `slzb_device_ble_proxy_connected` |
| `ambilight` | object | | LED ring settings (Ultima) | not exported |
| `disable_leds`, `night_mode`, `auto_zigbee` | boolean | | Settings toggles | not exported |

`lte_state` values: 0 idle, 1 starting, 2 connecting, 3 connected, 4 error,
5 no_signal.

`ble.state` values: 0 idle, 1 starting, 2 started, 3 error, 4 stopping,
5 stopped.

The SLZB-06 sends a subset of these keys (no `psram_usage`, `socket2_uptime`,
`socket3_uptime`, `otbr_uptime`, `lte_*`, `ble` nor `ambilight` in the
pysmlight sample). The exporter only creates a series for a key that was sent.

### Relation to the Legacy API

pysmlight uses `/ha_info` to choose its API: a device that answers it is
driven through `/api2`, and pysmlight falls back to `/api` only when
`/ha_info` answers 404 or `URL NOT FOUND`. The exporter keeps reading the
legacy `/api?action=0` endpoint and treats `/ha_*` as an extra source. When
both answer, existing metrics keep their legacy values; `/ha_sensors` only
feeds `slzb_device_temperature_celsius` and `slzb_device_uptime_seconds` in a
cycle where the legacy endpoint gave no data, and `/ha_info` `sw_version`
only replaces the legacy `VERSION` for `slzb_firmware_current_version` in
that case.

### Observed on SLZB-Ultima3 v3.4.5.dev0

Behaviour of a real SLZB-Ultima3 running SLZB-OS v3.4.5.dev0:

- The legacy `/api?action=N` endpoint answers 200 with the same gzipped SPA
  HTML page for every action, and sends no `respValuesArr` header. The exporter
  therefore treats the legacy source as absent and relies on `/ha_*`.
- `/api2?action=0&page=0` does carry a `respValuesArr` header, but with
  different keys (`BUILD`, `DEVICE`, `zbHw`, `zbHw2`, `zbHw3`, `uptime`, and
  more). `uptime` comes in a form like `[U_time]{^}  0{^}09{^}03`. The
  exporter does not parse this header.
- The device serves a native `GET /metrics` with only five series:
  `smlight_socket_clients`, `smlight_socket_uptime`, `smlight_device_uptime`
  (in milliseconds), `smlight_device_temp` and `smlight_free_heap`.
- Radio 2 is reported in `/ha_info` even without the Z-Wave addon, with
  `zb_version` 0 and `zb_type` 0. The exporter drops it, like pysmlight does.

## Notes

- All GET operations are safe and don't modify device state
- Some endpoints return gzipped content that needs decompression
- The device appears to use a simple action/page parameter system
- JSON responses are available for some endpoints
- Raw Zigbee data is available via action 9
- The web interface uses JavaScript to populate data-replace placeholders

## Future Exploration

The values behind the data-replace placeholders are served in the
`respValuesArr` response header of the legacy `/api?action=0` request on
SLZB-06 firmware, and by `/ha_info` and `/ha_sensors` on SLZB-OS. Remaining
unknowns:

- The meaning of the `/api2` `respValuesArr` keys on SLZB-OS v3
- Whether other SLZB-OS firmware versions behave like v3.4.5.dev0
