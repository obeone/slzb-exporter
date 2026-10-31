package metrics

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	promexporter_metrics "github.com/d0ugal/promexporter/metrics"
	"github.com/d0ugal/slzb-exporter/internal/slzbos"
	"github.com/prometheus/client_golang/prometheus"
)

// expectedOSMetric pairs an SLZB-OS gauge with the name and label set the
// plan (R5) requires for it.
type expectedOSMetric struct {
	vec    *prometheus.GaugeVec
	name   string
	labels []string
}

// expectedOSMetrics lists every R5 metric with its exact label names.
func expectedOSMetrics(m *SLZBOSMetrics) []expectedOSMetric {
	d := deviceLabel

	return []expectedOSMetric{
		{m.DeviceInfo, "slzb_device_info", []string{d, "model", "hostname", "sw_version", "hw_version", "mac"}},
		{m.RadioInfo, "slzb_radio_info", []string{d, "radio", "chip", "type", "channel", "firmware_version"}},
		{m.RAMSizeBytes, "slzb_device_ram_size_bytes", []string{d}},
		{m.PSRAMSizeBytes, "slzb_device_psram_size_bytes", []string{d}},
		{m.FSSizeBytes, "slzb_device_fs_size_bytes", []string{d}},
		{m.AddonPresent, "slzb_device_addon_present", []string{d, "addon"}},
		{m.BLEEnabled, "slzb_device_ble_enabled", []string{d}},
		{m.BLEProxyEnabled, "slzb_device_ble_proxy_enabled", []string{d}},
		{m.RadioTemperature, "slzb_radio_temperature_celsius", []string{d, "sensor"}},
		{m.SocketClientUptime, "slzb_socket_client_uptime_seconds", []string{d, "socket"}},
		{m.SocketClientConnected, "slzb_socket_client_connected", []string{d, "socket"}},
		{m.OTBRUptime, "slzb_otbr_uptime_seconds", []string{d}},
		{m.NetworkConnected, "slzb_device_network_connected", []string{d, "interface"}},
		{m.RAMUsedBytes, "slzb_device_ram_used_bytes", []string{d}},
		{m.PSRAMUsedBytes, "slzb_device_psram_used_bytes", []string{d}},
		{m.FSUsedBytes, "slzb_device_fs_used_bytes", []string{d}},
		{m.RAMLargestFreeBlock, "slzb_device_ram_largest_free_block_bytes", []string{d}},
		{m.PSRAMLargestFreeBlock, "slzb_device_psram_largest_free_block_bytes", []string{d}},
		{m.BLEState, "slzb_device_ble_state", []string{d}},
		{m.BLEProxyConnected, "slzb_device_ble_proxy_connected", []string{d}},
		{m.VPNEnabled, "slzb_device_vpn_enabled", []string{d}},
		{m.LTEDetected, "slzb_device_lte_detected", []string{d}},
		{m.LTEState, "slzb_device_lte_state", []string{d}},
	}
}

// newTestRegistry builds an SLZBRegistry on a fresh promexporter registry.
func newTestRegistry(t *testing.T) *SLZBRegistry {
	t.Helper()

	return NewSLZBRegistry(promexporter_metrics.NewRegistry("test_exporter_info"))
}

// labelsFor returns a label map assigning value to every name in names.
func labelsFor(names []string, value string) prometheus.Labels {
	labels := prometheus.Labels{}
	for _, name := range names {
		labels[name] = value
	}

	return labels
}

// TestSLZBOSMetricsCoverEveryVec guards against a field added to
// SLZBOSMetrics without an R5 expectation, or left out of AllVecs.
func TestSLZBOSMetricsCoverEveryVec(t *testing.T) {
	m := newTestRegistry(t).OS

	if got, want := len(m.AllVecs()), len(expectedOSMetrics(m)); got != want {
		t.Fatalf("AllVecs() has %d vecs, expected table has %d", got, want)
	}

	if got, want := len(m.InfoVecs())+len(m.SensorVecs()), len(m.AllVecs()); got != want {
		t.Fatalf("InfoVecs+SensorVecs = %d, AllVecs = %d", got, want)
	}
}

// TestSLZBOSMetricsNamesAndLabels checks that every R5 metric is registered
// on the Prometheus registry under its exact name and label set, and that
// the UI metadata matches.
func TestSLZBOSMetricsNamesAndLabels(t *testing.T) {
	reg := newTestRegistry(t)
	infos := reg.GetMetricsInfo()

	for _, exp := range expectedOSMetrics(reg.OS) {
		t.Run(exp.name, func(t *testing.T) {
			// GetMetricWith fails when the label names differ from the vec's.
			gauge, err := exp.vec.GetMetricWith(labelsFor(exp.labels, "x"))
			if err != nil {
				t.Fatalf("label set %v rejected: %v", exp.labels, err)
			}

			gauge.Set(1)

			if n := gatheredSeries(t, reg.GetRegistry(), exp.name); n != 1 {
				t.Fatalf("gathered %d series for %s, want 1", n, exp.name)
			}

			idx := slices.IndexFunc(infos, func(i promexporter_metrics.MetricInfo) bool { return i.Name == exp.name })
			if idx < 0 {
				t.Fatalf("%s missing from metric info", exp.name)
			}

			if !slices.Equal(infos[idx].Labels, exp.labels) {
				t.Fatalf("metric info labels %v, want %v", infos[idx].Labels, exp.labels)
			}
		})
	}
}

// TestRadioInfoHelpListsEmittedTypeNames checks that the slzb_radio_info
// HELP text names every type label value the collector can emit, so a
// PromQL selector copied from /metrics actually matches.
func TestRadioInfoHelpListsEmittedTypeNames(t *testing.T) {
	infos := newTestRegistry(t).GetMetricsInfo()

	idx := slices.IndexFunc(infos, func(i promexporter_metrics.MetricInfo) bool { return i.Name == "slzb_radio_info" })
	if idx < 0 {
		t.Fatal("slzb_radio_info missing from metric info")
	}

	// -1..8 covers every zb_type code pysmlight names; code 3 has no name
	// and is skipped, as its label value is the bare code.
	for code := int64(-1); code <= 8; code++ {
		name := slzbos.ZBTypeName(code)
		if name == strconv.FormatInt(code, 10) {
			continue
		}

		if !strings.Contains(infos[idx].Help, name) {
			t.Errorf("slzb_radio_info HELP lacks emitted type %q: %s", name, infos[idx].Help)
		}
	}
}

// TestSLZBOSMetricsReset checks that each reset helper only removes the
// series of the targeted device and of the targeted source.
func TestSLZBOSMetricsReset(t *testing.T) {
	m := newTestRegistry(t).OS
	expected := expectedOSMetrics(m)

	for _, exp := range expected {
		for _, device := range []string{"a", "b"} {
			labels := labelsFor(exp.labels, "x")
			labels[deviceLabel] = device
			exp.vec.With(labels).Set(1)
		}
	}

	m.ResetInfo("a")
	assertSeriesCount(t, m.InfoVecs(), 1)
	assertSeriesCount(t, m.SensorVecs(), 2)

	m.ResetSensors("a")
	assertSeriesCount(t, m.SensorVecs(), 1)

	m.ResetAll("b")
	assertSeriesCount(t, m.AllVecs(), 0)
}

// assertSeriesCount fails the test when any vec does not hold want series.
func assertSeriesCount(t *testing.T, vecs []*prometheus.GaugeVec, want int) {
	t.Helper()

	for _, vec := range vecs {
		if got := collectedSeries(vec); got != want {
			t.Errorf("vec has %d series, want %d", got, want)
		}
	}
}

// gatheredSeries returns how many series of the named metric family the
// registry exposes. It stands in for testutil.GatherAndCount, which would pull
// a new module into go.mod.
func gatheredSeries(t *testing.T, reg *prometheus.Registry, name string) int {
	t.Helper()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	for _, family := range families {
		if family.GetName() == name {
			return len(family.GetMetric())
		}
	}

	return 0
}

// collectedSeries returns how many series a collector currently holds.
func collectedSeries(c prometheus.Collector) int {
	ch := make(chan prometheus.Metric)

	go func() {
		c.Collect(ch)
		close(ch)
	}()

	n := 0
	for range ch {
		n++
	}

	return n
}
