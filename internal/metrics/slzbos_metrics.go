package metrics

import (
	promexporter_metrics "github.com/d0ugal/promexporter/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SLZBOSMetrics holds the gauges fed by the SLZB-OS JSON endpoints
// (/ha_info and /ha_sensors). They live apart from the legacy metrics so the
// two data sources stay independent: every vec here is keyed by "device"
// first, which lets the collector clear one device's series with a single
// DeletePartialMatch per vec.
type SLZBOSMetrics struct {
	// Sourced from /ha_info.
	DeviceInfo      *prometheus.GaugeVec
	RadioInfo       *prometheus.GaugeVec
	RAMSizeBytes    *prometheus.GaugeVec
	PSRAMSizeBytes  *prometheus.GaugeVec
	FSSizeBytes     *prometheus.GaugeVec
	AddonPresent    *prometheus.GaugeVec
	BLEEnabled      *prometheus.GaugeVec
	BLEProxyEnabled *prometheus.GaugeVec

	// Sourced from /ha_sensors.
	RadioTemperature      *prometheus.GaugeVec
	SocketClientUptime    *prometheus.GaugeVec
	SocketClientConnected *prometheus.GaugeVec
	OTBRUptime            *prometheus.GaugeVec
	NetworkConnected      *prometheus.GaugeVec
	RAMUsedBytes          *prometheus.GaugeVec
	PSRAMUsedBytes        *prometheus.GaugeVec
	FSUsedBytes           *prometheus.GaugeVec
	RAMLargestFreeBlock   *prometheus.GaugeVec
	PSRAMLargestFreeBlock *prometheus.GaugeVec
	BLEState              *prometheus.GaugeVec
	BLEProxyConnected     *prometheus.GaugeVec
	VPNEnabled            *prometheus.GaugeVec
	LTEDetected           *prometheus.GaugeVec
	LTEState              *prometheus.GaugeVec
}

// deviceLabel is the leading label of every SLZB-OS metric.
const deviceLabel = "device"

// gaugeRegistrar creates gauge vecs on the Prometheus registry and records
// their metadata on the promexporter registry, mirroring the two-step
// pattern used in NewSLZBRegistry without repeating it for every metric.
type gaugeRegistrar struct {
	factory promauto.Factory
	base    *promexporter_metrics.Registry
}

// gauge registers a gauge vec named name, labelled by "device" followed by
// extraLabels, and records the same name, help and labels for the UI.
func (r gaugeRegistrar) gauge(name, help string, extraLabels ...string) *prometheus.GaugeVec {
	labels := append([]string{deviceLabel}, extraLabels...)

	vec := r.factory.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, labels)
	r.base.AddMetricInfo(name, help, labels)

	return vec
}

// NewSLZBOSMetrics registers every SLZB-OS gauge on factory and records its
// metadata on baseRegistry.
func NewSLZBOSMetrics(factory promauto.Factory, baseRegistry *promexporter_metrics.Registry) *SLZBOSMetrics {
	r := gaugeRegistrar{factory: factory, base: baseRegistry}
	m := &SLZBOSMetrics{}

	m.registerInfoGauges(r)
	m.registerSensorGauges(r)

	return m
}

// registerInfoGauges creates the gauges whose values come from /ha_info.
func (m *SLZBOSMetrics) registerInfoGauges(r gaugeRegistrar) {
	m.DeviceInfo = r.gauge("slzb_device_info",
		"SLZB-OS device identity from /ha_info (always 1, used for joining with labels)",
		"model", "hostname", "sw_version", "hw_version", "mac")
	m.RadioInfo = r.gauge("slzb_radio_info",
		"SLZB-OS radio chip identity, one series per radio (always 1). "+
			"type: coordinator, router, thread, zhub, Zwave-EU, Zwave-US, Zwave-ANZ, otbr or unknown; "+
			"channel: stable, dev, custom or unknown; firmware_version is empty when the firmware reports none",
		"radio", "chip", "type", "channel", "firmware_version")
	m.RAMSizeBytes = r.gauge("slzb_device_ram_size_bytes",
		"SLZB-OS device total RAM in bytes; overlaps the legacy slzb_device_heap_size_bytes")
	m.PSRAMSizeBytes = r.gauge("slzb_device_psram_size_bytes",
		"SLZB-OS device total PSRAM in bytes")
	m.FSSizeBytes = r.gauge("slzb_device_fs_size_bytes",
		"SLZB-OS device filesystem size in bytes")
	m.AddonPresent = r.gauge("slzb_device_addon_present",
		"SLZB-OS hardware add-on presence (1=present, 0=absent), addon is lte, poe, zwave or mic",
		"addon")
	m.BLEEnabled = r.gauge("slzb_device_ble_enabled",
		"SLZB-OS Bluetooth LE enabled in configuration (1=enabled, 0=disabled)")
	m.BLEProxyEnabled = r.gauge("slzb_device_ble_proxy_enabled",
		"SLZB-OS Bluetooth LE proxy enabled in configuration (1=enabled, 0=disabled)")
}

// registerSensorGauges creates the gauges whose values come from /ha_sensors.
func (m *SLZBOSMetrics) registerSensorGauges(r gaugeRegistrar) {
	m.RadioTemperature = r.gauge("slzb_radio_temperature_celsius",
		"SLZB-OS radio temperature in degrees Celsius, sensor is zb_temp or zb_temp2 as named by the firmware; "+
			"which radio chip each sensor belongs to is undocumented",
		"sensor")
	m.SocketClientUptime = r.gauge("slzb_socket_client_uptime_seconds",
		"SLZB-OS TCP socket client uptime in seconds per socket (0=no client); "+
			"socket 1 overlaps the legacy slzb_socket_uptime_seconds",
		"socket")
	m.SocketClientConnected = r.gauge("slzb_socket_client_connected",
		"SLZB-OS TCP socket client connected (1=connected, 0=no client) per socket; "+
			"socket 1 overlaps the legacy slzb_socket_connected",
		"socket")
	m.OTBRUptime = r.gauge("slzb_otbr_uptime_seconds",
		"SLZB-OS OpenThread border router uptime in seconds (0=not running)")
	m.NetworkConnected = r.gauge("slzb_device_network_connected",
		"SLZB-OS network interface connected (1=connected, 0=disconnected), interface is ethernet, wifi or vpn; "+
			"ethernet and wifi overlap the legacy slzb_device_ethernet_connected and slzb_device_wifi_connected",
		"interface")
	m.RAMUsedBytes = r.gauge("slzb_device_ram_used_bytes",
		"SLZB-OS device used RAM in bytes; overlaps the legacy slzb_device_heap_free_bytes and slzb_device_heap_size_bytes")
	m.PSRAMUsedBytes = r.gauge("slzb_device_psram_used_bytes",
		"SLZB-OS device used PSRAM in bytes")
	m.FSUsedBytes = r.gauge("slzb_device_fs_used_bytes",
		"SLZB-OS device used filesystem space in bytes")
	m.RAMLargestFreeBlock = r.gauge("slzb_device_ram_largest_free_block_bytes",
		"SLZB-OS device largest contiguous free RAM block in bytes (heap fragmentation signal)")
	m.PSRAMLargestFreeBlock = r.gauge("slzb_device_psram_largest_free_block_bytes",
		"SLZB-OS device largest contiguous free PSRAM block in bytes (heap fragmentation signal)")
	m.BLEState = r.gauge("slzb_device_ble_state",
		"SLZB-OS Bluetooth LE state (0=idle, 1=starting, 2=started, 3=error, 4=stopping, 5=stopped)")
	m.BLEProxyConnected = r.gauge("slzb_device_ble_proxy_connected",
		"SLZB-OS Bluetooth LE proxy client connected (1=connected, 0=disconnected)")
	m.VPNEnabled = r.gauge("slzb_device_vpn_enabled",
		"SLZB-OS VPN enabled (1=enabled, 0=disabled)")
	m.LTEDetected = r.gauge("slzb_device_lte_detected",
		"SLZB-OS LTE modem detected (1=detected, 0=not detected)")
	m.LTEState = r.gauge("slzb_device_lte_state",
		"SLZB-OS LTE connection state (0=idle, 1=starting, 2=connecting, 3=connected, 4=error, 5=no_signal)")
}

// InfoVecs returns the gauges sourced from /ha_info.
func (m *SLZBOSMetrics) InfoVecs() []*prometheus.GaugeVec {
	return []*prometheus.GaugeVec{
		m.DeviceInfo, m.RadioInfo, m.RAMSizeBytes, m.PSRAMSizeBytes,
		m.FSSizeBytes, m.AddonPresent, m.BLEEnabled, m.BLEProxyEnabled,
	}
}

// SensorVecs returns the gauges sourced from /ha_sensors.
func (m *SLZBOSMetrics) SensorVecs() []*prometheus.GaugeVec {
	return []*prometheus.GaugeVec{
		m.RadioTemperature, m.SocketClientUptime, m.SocketClientConnected,
		m.OTBRUptime, m.NetworkConnected, m.RAMUsedBytes, m.PSRAMUsedBytes,
		m.FSUsedBytes, m.RAMLargestFreeBlock, m.PSRAMLargestFreeBlock,
		m.BLEState, m.BLEProxyConnected, m.VPNEnabled,
		m.LTEDetected, m.LTEState,
	}
}

// AllVecs returns every SLZB-OS gauge, /ha_info ones first.
func (m *SLZBOSMetrics) AllVecs() []*prometheus.GaugeVec {
	return append(m.InfoVecs(), m.SensorVecs()...)
}

// ResetInfo deletes every /ha_info-sourced series of device, so a fact that
// vanished from the payload does not linger as a stale series.
func (m *SLZBOSMetrics) ResetInfo(device string) {
	resetDevice(m.InfoVecs(), device)
}

// ResetSensors deletes every /ha_sensors-sourced series of device.
func (m *SLZBOSMetrics) ResetSensors(device string) {
	resetDevice(m.SensorVecs(), device)
}

// ResetAll deletes every SLZB-OS series of device, used when the device stops
// serving /ha_info.
func (m *SLZBOSMetrics) ResetAll(device string) {
	resetDevice(m.AllVecs(), device)
}

// resetDevice deletes the series of device from each vec. Partial matching is
// needed because most vecs carry labels beyond "device".
func resetDevice(vecs []*prometheus.GaugeVec, device string) {
	match := prometheus.Labels{deviceLabel: device}
	for _, vec := range vecs {
		vec.DeletePartialMatch(match)
	}
}
