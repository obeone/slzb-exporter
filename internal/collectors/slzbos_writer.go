package collectors

import (
	"github.com/d0ugal/slzb-exporter/internal/metrics"
	"github.com/d0ugal/slzb-exporter/internal/slzbos"
	"github.com/prometheus/client_golang/prometheus"
)

// The writers below map one decoded /ha_* payload to gauges, one function per
// metric group. A gauge is only set when its source key was sent: an absent
// key leaves no series rather than a misleading zero, and the per-device
// reset done first (R8) removes series whose key vanished since last cycle.

// writeSLZBOSInfo replaces every /ha_info-sourced series of deviceName with
// the values of info.
func (sc *SLZBCollector) writeSLZBOSInfo(deviceName string, info *slzbos.Info) {
	m := sc.metrics.OS
	m.ResetInfo(deviceName)

	writeDeviceIdentity(m, deviceName, info)
	writeRadios(m, deviceName, info.Radios)
	writeMemorySizes(m, deviceName, info)
	writeAddons(m, deviceName, info.Addons)
	writeBLEFeatures(m, deviceName, info.BLE)
}

// writeSLZBOSSensors replaces every /ha_sensors-sourced series of deviceName
// with the values of sensors. The shared legacy gauges are only fed when the
// legacy step produced no data, so one fact never has two competing sources.
func (sc *SLZBCollector) writeSLZBOSSensors(deviceName string, sensors *slzbos.Sensors, legacyOK bool) {
	m := sc.metrics.OS
	m.ResetSensors(deviceName)

	if !legacyOK {
		sc.writeSharedLegacyGauges(deviceName, sensors)
	}

	writeRadioTemperatures(m, deviceName, sensors)
	writeSocketClients(m, deviceName, sensors)
	writeOTBR(m, deviceName, sensors.OTBRUptime)
	writeNetworkStatus(m, deviceName, sensors)
	writeMemoryUsage(m, deviceName, sensors)
	writeBLESession(m, deviceName, sensors.BLE)
	writeLTE(m, deviceName, sensors)
}

// writeSharedLegacyGauges feeds the pre-existing temperature and uptime
// gauges from /ha_sensors, for devices whose legacy endpoint gave no data.
func (sc *SLZBCollector) writeSharedLegacyGauges(deviceName string, sensors *slzbos.Sensors) {
	labels := deviceLabels(deviceName)
	setFlex(sc.metrics.SLZBDeviceTemp, labels, sensors.ESP32Temp, 1)
	setFlex(sc.metrics.SLZBUptime, labels, sensors.Uptime, 1)
}

// writeDeviceIdentity sets slzb_device_info. Label values are the raw
// firmware strings (no pysmlight cosmetic rewrites).
func writeDeviceIdentity(m *metrics.SLZBOSMetrics, deviceName string, info *slzbos.Info) {
	m.DeviceInfo.With(deviceLabels(deviceName,
		"model", info.Model,
		"hostname", info.Hostname,
		"sw_version", info.SWVersion,
		"hw_version", info.HWVersion.String(),
		"mac", info.MAC,
	)).Set(1)
}

// writeRadios sets one slzb_radio_info series per normalised radio.
func writeRadios(m *metrics.SLZBOSMetrics, deviceName string, radios []slzbos.Radio) {
	for i := range radios {
		radio := &radios[i]
		m.RadioInfo.With(deviceLabels(deviceName,
			"radio", radio.Index(),
			"chip", radio.HW,
			"type", radio.TypeName(),
			"channel", radio.ChannelName(),
			"firmware_version", radio.FirmwareVersion(),
		)).Set(1)
	}
}

// writeMemorySizes sets the RAM, PSRAM and filesystem capacities in bytes.
func writeMemorySizes(m *metrics.SLZBOSMetrics, deviceName string, info *slzbos.Info) {
	labels := deviceLabels(deviceName)
	setFlex(m.RAMSizeBytes, labels, info.RAMTotalKB, bytesPerKB)
	setFlex(m.PSRAMSizeBytes, labels, info.PSRAMTotalKB, bytesPerKB)
	setFlex(m.FSSizeBytes, labels, info.FSTotalKB, bytesPerKB)
}

// writeAddons sets one slzb_device_addon_present series per addon reported.
func writeAddons(m *metrics.SLZBOSMetrics, deviceName string, addons slzbos.Addons) {
	for name, present := range addons {
		m.AddonPresent.With(deviceLabels(deviceName, "addon", name)).Set(boolToFloat(present))
	}
}

// writeBLEFeatures sets the configured BLE features, when reported.
func writeBLEFeatures(m *metrics.SLZBOSMetrics, deviceName string, ble *slzbos.BLEFeatures) {
	if ble == nil {
		return
	}

	labels := deviceLabels(deviceName)
	setBool(m.BLEEnabled, labels, ble.Enabled)
	setBool(m.BLEProxyEnabled, labels, ble.ProxyEnabled)
}

// writeRadioTemperatures sets slzb_radio_temperature_celsius per sensor key.
func writeRadioTemperatures(m *metrics.SLZBOSMetrics, deviceName string, sensors *slzbos.Sensors) {
	setFlex(m.RadioTemperature, deviceLabels(deviceName, "sensor", "zb_temp"), sensors.ZBTemp, 1)
	setFlex(m.RadioTemperature, deviceLabels(deviceName, "sensor", "zb_temp2"), sensors.ZBTemp2, 1)
}

// writeSocketClients sets the uptime and connected gauges of each TCP socket.
// The firmware reports 0 seconds when no client is connected.
func writeSocketClients(m *metrics.SLZBOSMetrics, deviceName string, sensors *slzbos.Sensors) {
	sockets := map[string]*slzbos.FlexNumber{
		"1": sensors.SocketUptime,
		"2": sensors.Socket2Uptime,
		"3": sensors.Socket3Uptime,
	}

	for socket, uptime := range sockets {
		if !uptime.IsSet() {
			continue
		}

		labels := deviceLabels(deviceName, "socket", socket)
		m.SocketClientUptime.With(labels).Set(uptime.Float64())
		m.SocketClientConnected.With(labels).Set(boolToFloat(uptime.Float64() > 0))
	}
}

// writeOTBR sets the OpenThread border router uptime, when reported.
func writeOTBR(m *metrics.SLZBOSMetrics, deviceName string, uptime *slzbos.FlexNumber) {
	setFlex(m.OTBRUptime, deviceLabels(deviceName), uptime, 1)
}

// writeNetworkStatus sets the per-interface link state and the VPN switch.
func writeNetworkStatus(m *metrics.SLZBOSMetrics, deviceName string, sensors *slzbos.Sensors) {
	setBool(m.NetworkConnected, deviceLabels(deviceName, "interface", "ethernet"), sensors.Ethernet)
	setBool(m.NetworkConnected, deviceLabels(deviceName, "interface", "wifi"), sensors.WifiConnected)
	setBool(m.NetworkConnected, deviceLabels(deviceName, "interface", "vpn"), sensors.VPNStatus)
	setBool(m.VPNEnabled, deviceLabels(deviceName), sensors.VPNEnabled)
}

// writeMemoryUsage sets the RAM, PSRAM and filesystem usage in bytes, plus
// the largest free RAM and PSRAM blocks when the firmware reports them.
func writeMemoryUsage(m *metrics.SLZBOSMetrics, deviceName string, sensors *slzbos.Sensors) {
	labels := deviceLabels(deviceName)
	setFlex(m.RAMUsedBytes, labels, sensors.RAMUsageKB, bytesPerKB)
	setFlex(m.PSRAMUsedBytes, labels, sensors.PSRAMUsageKB, bytesPerKB)
	setFlex(m.FSUsedBytes, labels, sensors.FSUsedKB, bytesPerKB)
	setFlex(m.RAMLargestFreeBlock, labels, sensors.RAMLargestFreeBlockKB, bytesPerKB)
	setFlex(m.PSRAMLargestFreeBlock, labels, sensors.PSRAMLargestFreeBlockKB, bytesPerKB)
}

// writeBLESession sets the live BLE stack state, when reported.
func writeBLESession(m *metrics.SLZBOSMetrics, deviceName string, ble *slzbos.BLESession) {
	if ble == nil {
		return
	}

	labels := deviceLabels(deviceName)
	setFlex(m.BLEState, labels, ble.State, 1)
	setBool(m.BLEProxyConnected, labels, ble.ProxyConnected)
}

// writeLTE sets the LTE modem detection and state, when reported.
func writeLTE(m *metrics.SLZBOSMetrics, deviceName string, sensors *slzbos.Sensors) {
	labels := deviceLabels(deviceName)
	setBool(m.LTEDetected, labels, sensors.LTEDetect)
	setFlex(m.LTEState, labels, sensors.LTEState, 1)
}

// deviceLabels builds the label set {"device": deviceName} extended with the
// given name/value pairs.
func deviceLabels(deviceName string, pairs ...string) prometheus.Labels {
	labels := prometheus.Labels{"device": deviceName}
	for i := 0; i+1 < len(pairs); i += 2 {
		labels[pairs[i]] = pairs[i+1]
	}

	return labels
}

// setFlex sets vec{labels} to value*scale when value was sent.
func setFlex(vec *prometheus.GaugeVec, labels prometheus.Labels, value *slzbos.FlexNumber, scale float64) {
	if !value.IsSet() {
		return
	}

	vec.With(labels).Set(value.Float64() * scale)
}

// setBool sets vec{labels} to 1 or 0 when value was sent.
func setBool(vec *prometheus.GaugeVec, labels prometheus.Labels, value *bool) {
	if value == nil {
		return
	}

	vec.With(labels).Set(boolToFloat(*value))
}

// boolToFloat maps true to 1 and false to 0 for gauge values.
func boolToFloat(b bool) float64 {
	if b {
		return 1
	}

	return 0
}
