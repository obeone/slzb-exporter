package collectors

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// These tests drive whole collection cycles against a fake device and cover
// the plan's integration cases a) to i): how the legacy /api step and the
// SLZB-OS /ha_* step combine into reachability, error accounting and series.

// legacyOnlyGoldenNames is the exported slzb_* family set of a legacy-only
// device after one cycle, captured on main (before SLZB-OS support) against
// legacyOnlyRoutes. Error counters are absent because nothing failed.
var legacyOnlyGoldenNames = []string{
	"slzb_api_response_time_seconds",
	"slzb_collection_duration_seconds",
	"slzb_config_backup_status",
	"slzb_config_files",
	"slzb_config_last_backup_timestamp",
	"slzb_device_connected",
	"slzb_device_ethernet_connected",
	"slzb_device_heap_free_bytes",
	"slzb_device_heap_ratio",
	"slzb_device_heap_size_bytes",
	"slzb_device_operational_mode",
	"slzb_device_reachable",
	"slzb_device_temperature_celsius",
	"slzb_device_uptime_seconds",
	"slzb_device_wifi_connected",
	"slzb_device_wifi_rssi_dbm",
	"slzb_firmware_current_version",
	"slzb_firmware_last_check_timestamp",
	"slzb_firmware_update_available",
	"slzb_http_requests_total",
	"slzb_last_collection_timestamp",
	"slzb_socket_connected",
	"slzb_socket_uptime_seconds",
}

// Values of the fixtures, asserted by several cases.
const (
	ultima3SWVersion = "v3.2.6.dev2"
	legacyVersion    = "v2.7.2"
	legacyTemp       = 45.5
	// legacyUptime is "7 d 16:47:19" in seconds.
	legacyUptime = 7*86400 + 16*3600 + 47*60 + 19
)

// assertReachable checks slzb_device_reachable and slzb_device_connected.
func assertReachable(t *testing.T, tc *testCollector, want float64) {
	t.Helper()

	assertValue(t, tc.registry.SLZBDeviceReachable, tc.labels(), want)
	assertValue(t, tc.registry.SLZBConnected, tc.labels(), want)
}

// assertNotUnreachable checks that no device_unreachable error was counted.
func assertNotUnreachable(t *testing.T, tc *testCollector) {
	t.Helper()

	assertNoSeries(t, tc.registry.SLZBCollectionErrors, tc.labels("error_type", "device_unreachable"))
}

// assertFirmwareVersion checks that slzb_firmware_current_version holds a
// single series, for version.
func assertFirmwareVersion(t *testing.T, tc *testCollector, version string) {
	t.Helper()

	assertSeriesCount(t, tc.registry.SLZBFirmwareCurrentVersion, 1)
	assertValue(t, tc.registry.SLZBFirmwareCurrentVersion, tc.labels("version", version), 1)
}

// assertUltima3Collected checks one cycle of an Ultima3 whose legacy
// endpoint is absent: reachable through /ha_* alone, quiet about the legacy
// endpoint, no file-list request, and every /ha_* fact exported.
func assertUltima3Collected(t *testing.T, tc *testCollector, dev *fakeDevice) {
	t.Helper()

	assertReachable(t, tc, 1)
	assertNotUnreachable(t, tc)
	assertNoSeries(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", "0"))
	assertNoSeries(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", "4"))

	if n := dev.requestCount(routeConfig); n != 0 {
		t.Errorf("%s requested %d times, want 0 without legacy data", routeConfig, n)
	}

	assertValue(t, tc.registry.SLZBDeviceTemp, tc.labels(), 39.44)
	assertValue(t, tc.registry.SLZBUptime, tc.labels(), 700)
	assertFirmwareVersion(t, tc, ultima3SWVersion)

	osm := tc.registry.OS
	assertSeriesCount(t, osm.RadioInfo, 3)
	assertSeriesCount(t, osm.RadioTemperature, 2)
	assertSeriesCount(t, osm.SocketClientUptime, 3)
	assertValue(t, osm.PSRAMUsedBytes, tc.labels(), 28*1024)
	assertValue(t, osm.RAMLargestFreeBlock, tc.labels(), 31*1024)
	assertValue(t, osm.PSRAMLargestFreeBlock, tc.labels(), 1984*1024)
	// The fixture's integer addons.addon_type must neither break /ha_info
	// nor become an addon series: only the four boolean addons remain.
	assertSeriesCount(t, osm.AddonPresent, 4)
	assertValue(t, osm.AddonPresent, tc.labels("addon", "zwave"), 1)
	assertNoSeries(t, osm.AddonPresent, tc.labels("addon", "addon_type"))
	assertValue(t, osm.LTEState, tc.labels(), 3)
	assertValue(t, osm.BLEState, tc.labels(), 2)
}

// TestUltima3WithoutLegacyEndpointIsCollectedFromSLZBOS covers case a): the
// legacy endpoint answers 404 and everything comes from /ha_*.
func TestUltima3WithoutLegacyEndpointIsCollectedFromSLZBOS(t *testing.T) {
	dev := newFakeDevice(t, ultima3Routes(t))
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	assertUltima3Collected(t, tc, dev)
}

// TestUltima3WithURLNotFoundLegacyBodyIsCollectedFromSLZBOS covers case a2):
// the legacy endpoint answers 200 "URL NOT FOUND" without respValuesArr.
func TestUltima3WithURLNotFoundLegacyBodyIsCollectedFromSLZBOS(t *testing.T) {
	routes := ultima3Routes(t)
	routes[routeLegacy] = fakeResponse{status: http.StatusOK, body: "URL NOT FOUND"}

	dev := newFakeDevice(t, routes)
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	assertUltima3Collected(t, tc, dev)
}

// TestLegacyOnlyDeviceExportsUnchangedMetricSet covers case b): a device
// without /ha_* exports exactly the metric names it did before SLZB-OS
// support, and the unsupported endpoint is never counted as an error.
func TestLegacyOnlyDeviceExportsUnchangedMetricSet(t *testing.T) {
	dev := newFakeDevice(t, legacyOnlyRoutes(t))
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	if got := tc.gatheredNames(t, "slzb_"); !slices.Equal(got, legacyOnlyGoldenNames) {
		t.Errorf("exported metric names differ from main\n got: %v\nwant: %v", got, legacyOnlyGoldenNames)
	}

	assertNoSeries(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", actionHAInfo))
	assertNoSeries(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", actionHASensors))

	if n := dev.requestCount(routeHASensors); n != 0 {
		t.Errorf("%s requested %d times, want 0 when /ha_info is unsupported", routeHASensors, n)
	}
}

// TestSLZB06CoreKeepsLegacyValuesAndAddsSLZBOSSeries covers case c): with
// both sources answering, shared gauges keep their legacy value and the new
// series appear only for keys the device sends.
func TestSLZB06CoreKeepsLegacyValuesAndAddsSLZBOSSeries(t *testing.T) {
	dev := newFakeDevice(t, slzb06CoreRoutes(t))
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	assertReachable(t, tc, 1)
	assertSeriesCount(t, tc.registry.SLZBDeviceTemp, 1)
	assertValue(t, tc.registry.SLZBDeviceTemp, tc.labels(), legacyTemp)
	assertValue(t, tc.registry.SLZBUptime, tc.labels(), legacyUptime)
	assertFirmwareVersion(t, tc, legacyVersion)

	osm := tc.registry.OS
	assertValue(t, osm.SocketClientUptime, tc.labels("socket", "1"), 0)
	assertValue(t, osm.NetworkConnected, tc.labels("interface", "ethernet"), 1)
	assertValue(t, osm.NetworkConnected, tc.labels("interface", "wifi"), 0)
	assertValue(t, osm.RAMUsedBytes, tc.labels(), 91*1024)

	assertNoSeries(t, osm.SocketClientUptime, tc.labels("socket", "2"))

	absentOnSLZB06 := []prometheus.Collector{
		osm.LTEState, osm.LTEDetected, osm.BLEState, osm.BLEProxyConnected, osm.OTBRUptime,
		osm.RAMLargestFreeBlock, osm.PSRAMLargestFreeBlock,
	}
	for _, absent := range absentOnSLZB06 {
		assertSeriesCount(t, absent, 0)
	}
}

// TestSensorsServerErrorIsCountedAndKeepsInfo covers case d): a 500 on
// /ha_sensors is a counted error that does not undo the /ha_info series.
func TestSensorsServerErrorIsCountedAndKeepsInfo(t *testing.T) {
	routes := ultima3Routes(t)
	routes[routeHASensors] = fakeResponse{status: http.StatusInternalServerError, body: "oops"}

	dev := newFakeDevice(t, routes)
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	assertValue(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", actionHASensors, "error_type", "http_error"), 1)
	assertSeriesCount(t, tc.registry.OS.DeviceInfo, 1)
	assertReachable(t, tc, 1)
}

// TestSWVersionChangeLeavesSingleInfoAndFirmwareSeries covers case e): an
// Ultima3 firmware upgrade replaces the identity and firmware series instead
// of leaving the old version behind.
func TestSWVersionChangeLeavesSingleInfoAndFirmwareSeries(t *testing.T) {
	routes := ultima3Routes(t)
	dev := newFakeDevice(t, routes)
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	upgraded := editPayload(t, readOSFixture(t, "ha_info_ultima3.json"), "Info", func(info map[string]any) {
		info["sw_version"] = "v3.2.7"
	})
	dev.setRoute(routeHAInfo, okResponse(upgraded))

	tc.collect()

	assertSeriesCount(t, tc.registry.OS.DeviceInfo, 1)
	assertValue(t, tc.registry.OS.DeviceInfo, tc.labels("sw_version", "v3.2.7"), 1)
	assertFirmwareVersion(t, tc, "v3.2.7")
}

// TestLegacyVersionChangeLeavesSingleFirmwareSeries covers case e2): on a
// legacy-only SLZB-06, a VERSION change after OTA no longer leaves a stale
// firmware series (intended behaviour change).
func TestLegacyVersionChangeLeavesSingleFirmwareSeries(t *testing.T) {
	dev := newFakeDevice(t, legacyOnlyRoutes(t))
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()
	assertFirmwareVersion(t, tc, legacyVersion)

	dev.setRoute(routeLegacy, legacyValuesResponse(legacyHeader(t, map[string]string{"VERSION": "v2.7.5"})))

	tc.collect()

	assertFirmwareVersion(t, tc, "v2.7.5")
}

// TestVanishedSensorKeyDeletesItsSeries covers case f): a key that
// disappears from /ha_sensors takes its series with it, while the other
// sensor series stay.
func TestVanishedSensorKeyDeletesItsSeries(t *testing.T) {
	dev := newFakeDevice(t, ultima3Routes(t))
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()
	assertSeriesCount(t, tc.registry.OS.LTEState, 1)

	withoutLTE := editPayload(t, readOSFixture(t, "ha_sensors_ultima3.json"), "Sensors", func(sensors map[string]any) {
		delete(sensors, "lte_state")
	})
	dev.setRoute(routeHASensors, okResponse(withoutLTE))

	tc.collect()

	assertSeriesCount(t, tc.registry.OS.LTEState, 0)
	assertSeriesCount(t, tc.registry.OS.LTEDetected, 1)
}

// TestLegacyTransportErrorSkipsSLZBOS covers case g): with no answer at all
// on the legacy endpoint, /ha_* is not tried and the device is unreachable.
func TestLegacyTransportErrorSkipsSLZBOS(t *testing.T) {
	routes := ultima3Routes(t)
	routes[routeLegacy] = fakeResponse{dropConnection: true}

	dev := newFakeDevice(t, routes)
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	for _, route := range []string{routeHAInfo, routeHASensors} {
		if n := dev.requestCount(route); n != 0 {
			t.Errorf("%s requested %d times after a legacy transport error, want 0", route, n)
		}
	}

	assertReachable(t, tc, 0)
	assertValue(t, tc.registry.SLZBCollectionErrors, tc.labels("error_type", "device_unreachable"), 1)
}

// TestLegacyOnlyWithoutHeaderKeepsHistoricalBehaviour covers case h): without
// /ha_*, a 200 lacking respValuesArr is still reachable and still leads to
// the file-list request.
func TestLegacyOnlyWithoutHeaderKeepsHistoricalBehaviour(t *testing.T) {
	routes := legacyOnlyRoutes(t)
	routes[routeLegacy] = fakeResponse{status: http.StatusOK, body: "<html></html>"}
	routes[routeHAInfo] = fakeResponse{status: http.StatusNotFound, body: "Not found"}

	dev := newFakeDevice(t, routes)
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	assertReachable(t, tc, 1)
	assertNotUnreachable(t, tc)
	assertSeriesCount(t, tc.registry.SLZBHTTPErrorsTotal, 0)
	assertSeriesCount(t, tc.registry.OS.DeviceInfo, 0)

	if n := dev.requestCount(routeConfig); n != 1 {
		t.Errorf("%s requested %d times, want 1", routeConfig, n)
	}
}

// TestLegacyServerErrorWithSLZBOSStaysReachable covers case i): a legacy 500
// is still counted, but /ha_* keeps the device reachable.
func TestLegacyServerErrorWithSLZBOSStaysReachable(t *testing.T) {
	routes := ultima3Routes(t)
	routes[routeLegacy] = fakeResponse{status: http.StatusInternalServerError, body: "oops"}

	dev := newFakeDevice(t, routes)
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	assertReachable(t, tc, 1)
	assertNotUnreachable(t, tc)
	assertValue(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", "0", "error_type", "http_error"), 1)
}

// TestUltima3RealDeviceV345ServesSPAForLegacyEndpoint covers the real
// SLZB-Ultima3 v3.4.5.dev0: /api?action=0 answers 200 with the gzipped SPA
// page and no respValuesArr header, so the legacy source counts as absent and
// everything comes from /ha_*, without error or file-list request.
func TestUltima3RealDeviceV345ServesSPAForLegacyEndpoint(t *testing.T) {
	var page bytes.Buffer

	zw := gzip.NewWriter(&page)
	if _, err := zw.Write([]byte("<html><body>SLZB-OS SPA</body></html>")); err != nil {
		t.Fatalf("gzip page: %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	dev := newFakeDevice(t, map[string]fakeResponse{
		routeLegacy: {
			status:  http.StatusOK,
			body:    page.String(),
			headers: map[string]string{"Content-Encoding": "gzip", "Content-Type": "text/html"},
		},
		routeHAInfo:    okResponse(readOSFixture(t, "ha_info_ultima3_v3.4.5.json")),
		routeHASensors: okResponse(readOSFixture(t, "ha_sensors_ultima3_v3.4.5.json")),
	})
	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	assertReachable(t, tc, 1)
	assertNotUnreachable(t, tc)
	assertSeriesCount(t, tc.registry.SLZBHTTPErrorsTotal, 0)
	assertNoSeries(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", "4"))

	if n := dev.requestCount(routeConfig); n != 0 {
		t.Errorf("%s requested %d times, want 0 without legacy data", routeConfig, n)
	}

	osm := tc.registry.OS
	assertSeriesCount(t, osm.RadioInfo, 2)
	assertValue(t, osm.RAMLargestFreeBlock, tc.labels(), 30*1024)
}
