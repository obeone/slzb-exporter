package slzbos

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

// wantRadio is the expected label view of one normalised radio.
type wantRadio struct {
	index, hw, version, typeName, channel string
}

// TestParseInfoRadios checks radio normalisation and enum naming across the
// Ultima3, Ultima3 without Z-Wave and single-radio fixtures.
func TestParseInfoRadios(t *testing.T) {
	ultimaFirstTwo := []wantRadio{
		{index: "0", hw: "EFR32MG24", version: "20251218", typeName: "otbr", channel: "stable"},
		{index: "1", hw: "CC2674P10", version: "20250325", typeName: "thread", channel: "dev"},
	}

	tests := []struct {
		name    string
		fixture string
		want    []wantRadio
	}{
		{
			name:    "Ultima3 with zwave keeps three radios and radio 2 at -1 becomes custom without version",
			fixture: "ha_info_ultima3.json",
			want: append(append([]wantRadio{}, ultimaFirstTwo...),
				wantRadio{index: "2", hw: "EFR32ZG23", version: "", typeName: "unknown", channel: "custom"}),
		},
		{
			name:    "Ultima3 without zwave addon is truncated to two radios",
			fixture: "ha_info_ultima3_no_zwave.json",
			want:    ultimaFirstTwo,
		},
		{
			name:    "firmware without radios array gets radio 0 synthesised from top-level fields",
			fixture: "ha_info_no_radios.json",
			want:    []wantRadio{{index: "0", hw: "CC2674P10", version: "20240315", typeName: "coordinator", channel: ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseInfo(readFixture(t, tt.fixture))
			if err != nil {
				t.Fatalf("ParseInfo: %v", err)
			}

			assertRadios(t, info.Radios, tt.want)
		})
	}
}

// TestParseInfoKeepsRawLabels checks that device-level fields are exposed as
// sent by the firmware, without pysmlight's cosmetic rewrites.
func TestParseInfoKeepsRawLabels(t *testing.T) {
	info, err := ParseInfo(readFixture(t, "ha_info_ultima3.json"))
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}

	checks := map[string][2]string{
		"model":      {info.Model, "SLZB-Ultima3"},
		"hostname":   {info.Hostname, "SLZB-ULTIMA3"},
		"sw_version": {info.SWVersion, "v3.2.6.dev2"},
		"hw_version": {info.HWVersion.String(), "104"},
		"mac":        {info.MAC, "00:11:22:33:44:55"},
		"fw_channel": {info.FWChannel, "dev"},
		"psram":      {info.PSRAMTotalKB.String(), "2015"},
	}
	for field, pair := range checks {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q", field, pair[0], pair[1])
		}
	}

	if !info.Addons.Has("zwave") || info.Addons.Has("lte") {
		t.Errorf("addons = %v, want zwave fitted and lte absent", info.Addons)
	}

	if info.BLE == nil || info.BLE.Enabled == nil || !*info.BLE.Enabled {
		t.Errorf("ble.ble_enabled not decoded as true: %+v", info.BLE)
	}
}

// TestParseInfoSkipsNonBooleanAddons checks that a non-boolean member of
// "addons", such as the integer addon_type sent by real Ultima3 firmware,
// is ignored instead of making the whole /ha_info payload malformed.
func TestParseInfoSkipsNonBooleanAddons(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Addons
	}{
		{
			name: "integer addon_type is skipped and booleans are kept",
			body: `{"Info":{"addons":{"zwave":true,"lte":false,"addon_type":2}}}`,
			want: Addons{"zwave": true, "lte": false},
		},
		{
			name: "string and object members are skipped",
			body: `{"Info":{"addons":{"mic":true,"kind":"x","extra":{"a":1}}}}`,
			want: Addons{"mic": true},
		},
		{
			name: "null addons decode as no addon",
			body: `{"Info":{"addons":null}}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseInfo([]byte(tt.body))
			if err != nil {
				t.Fatalf("ParseInfo: %v", err)
			}

			if !maps.Equal(info.Addons, tt.want) {
				t.Errorf("addons = %v, want %v", info.Addons, tt.want)
			}
		})
	}
}

// TestParseInfoRejectsNonObjectAddons checks that an "addons" value that is
// not an object at all still marks the payload as malformed.
func TestParseInfoRejectsNonObjectAddons(t *testing.T) {
	_, err := ParseInfo([]byte(`{"Info":{"addons":[true]}}`))
	if !errors.Is(err, ErrMalformedObject) {
		t.Errorf("error = %v, want %v", err, ErrMalformedObject)
	}
}

// TestParseInfoUltima3FixtureIgnoresAddonType checks the Ultima3 fixture,
// which carries addon_type like the real device: parsing succeeds and only
// the four boolean addons remain.
func TestParseInfoUltima3FixtureIgnoresAddonType(t *testing.T) {
	info, err := ParseInfo(readFixture(t, "ha_info_ultima3.json"))
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}

	want := Addons{"lte": false, "poe": false, "zwave": true, "mic": false}
	if !maps.Equal(info.Addons, want) {
		t.Errorf("addons = %v, want %v", info.Addons, want)
	}
}

// TestParseSensorsUltima3 checks string temperatures and Ultima-only keys.
func TestParseSensorsUltima3(t *testing.T) {
	s, err := ParseSensors(readFixture(t, "ha_sensors_ultima3.json"))
	if err != nil {
		t.Fatalf("ParseSensors: %v", err)
	}

	floats := map[string][2]float64{
		"esp32_temp":     {s.ESP32Temp.Float64(), 39.44},
		"zb_temp":        {s.ZBTemp.Float64(), 35.76},
		"zb_temp2":       {s.ZBTemp2.Float64(), 34.20},
		"uptime":         {s.Uptime.Float64(), 700},
		"socket2_uptime": {s.Socket2Uptime.Float64(), 123},
		"psram_usage":    {s.PSRAMUsageKB.Float64(), 28},
		"lte_state":      {s.LTEState.Float64(), 3},
		"ram_lfb":        {s.RAMLargestFreeBlockKB.Float64(), 31},
		"psram_lfb":      {s.PSRAMLargestFreeBlockKB.Float64(), 1984},
	}
	for field, pair := range floats {
		if pair[0] != pair[1] {
			t.Errorf("%s = %v, want %v", field, pair[0], pair[1])
		}
	}

	if s.BLE == nil || s.BLE.State.Float64() != 2 || s.BLE.ProxyConnected == nil || !*s.BLE.ProxyConnected {
		t.Errorf("ble session not decoded: %+v", s.BLE)
	}

	if s.Ethernet == nil || !*s.Ethernet || s.LTEDetect == nil || *s.LTEDetect {
		t.Errorf("ethernet/lte_detect not decoded: ethernet=%v lte_detect=%v", s.Ethernet, s.LTEDetect)
	}
}

// TestParseInfoUltima3RealDeviceV345 checks the sanitised capture of a real
// SLZB-Ultima3 on v3.4.5.dev0: radio 2 reported without the Z-Wave addon is
// dropped, addon_type is ignored and extra keys do not break decoding.
func TestParseInfoUltima3RealDeviceV345(t *testing.T) {
	info, err := ParseInfo(readFixture(t, "ha_info_ultima3_v3.4.5.json"))
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}

	assertRadios(t, info.Radios, []wantRadio{
		{index: "0", hw: "EFR32MG24", version: "20251218", typeName: "otbr", channel: "stable"},
		{index: "1", hw: "CC2674P10", version: "20260311", typeName: "coordinator", channel: "dev"},
	})

	wantAddons := Addons{"zwave": false, "lte": false, "poe": false, "mic": false}
	if !maps.Equal(info.Addons, wantAddons) {
		t.Errorf("addons = %v, want %v", info.Addons, wantAddons)
	}

	if got := info.HWVersion.String(); got != "104" {
		t.Errorf("hw_version = %q, want 104", got)
	}

	if info.SWVersion != "v3.4.5.dev0" {
		t.Errorf("sw_version = %q, want v3.4.5.dev0", info.SWVersion)
	}
}

// TestParseSensorsUltima3RealDeviceV345 checks the sanitised /ha_sensors
// capture of the same real device.
func TestParseSensorsUltima3RealDeviceV345(t *testing.T) {
	s, err := ParseSensors(readFixture(t, "ha_sensors_ultima3_v3.4.5.json"))
	if err != nil {
		t.Fatalf("ParseSensors: %v", err)
	}

	floats := map[string][2]float64{
		"esp32_temp":     {s.ESP32Temp.Float64(), 45.80},
		"ram_lfb":        {s.RAMLargestFreeBlockKB.Float64(), 30},
		"psram_lfb":      {s.PSRAMLargestFreeBlockKB.Float64(), 1471},
		"socket2_uptime": {s.Socket2Uptime.Float64(), 2972},
		"otbr_uptime":    {s.OTBRUptime.Float64(), 32595},
		"lte_state":      {s.LTEState.Float64(), 0},
	}
	for field, pair := range floats {
		if pair[0] != pair[1] {
			t.Errorf("%s = %v, want %v", field, pair[0], pair[1])
		}
	}

	if s.BLE == nil || s.BLE.State.Float64() != 2 {
		t.Errorf("ble state not decoded as 2: %+v", s.BLE)
	}
}

// TestParseSensorsSLZB06LeavesUltimaKeysNil checks that keys absent on a
// single-radio device stay nil instead of reading as zero.
func TestParseSensorsSLZB06LeavesUltimaKeysNil(t *testing.T) {
	s, err := ParseSensors(readFixture(t, "ha_sensors_slzb06.json"))
	if err != nil {
		t.Fatalf("ParseSensors: %v", err)
	}

	absent := map[string]bool{
		"socket2_uptime": s.Socket2Uptime == nil,
		"socket3_uptime": s.Socket3Uptime == nil,
		"otbr_uptime":    s.OTBRUptime == nil,
		"psram_usage":    s.PSRAMUsageKB == nil,
		"lte_state":      s.LTEState == nil,
		"lte_detect":     s.LTEDetect == nil,
		"vpn_enabled":    s.VPNEnabled == nil,
		"ble":            s.BLE == nil,
		"ram_lfb":        s.RAMLargestFreeBlockKB == nil,
		"psram_lfb":      s.PSRAMLargestFreeBlockKB == nil,
	}
	for field, isNil := range absent {
		if !isNil {
			t.Errorf("%s should be nil on SLZB-06", field)
		}
	}

	if !s.SocketUptime.IsSet() || s.SocketUptime.Float64() != 0 {
		t.Errorf("socket_uptime should be set to 0, got %q", s.SocketUptime.String())
	}
}

// TestParseErrorsDistinguishMissingFromMalformed checks the error classes
// that the client turns into unsupported versus failed outcomes.
func TestParseErrorsDistinguishMissingFromMalformed(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		sensors bool
		want    error
	}{
		{name: "HTML body has no Info object", body: "<html>SLZB</html>", want: ErrMissingObject},
		{name: "JSON without Info key", body: `{"Sensors":{}}`, want: ErrMissingObject},
		{name: "null Info object", body: `{"Info":null}`, want: ErrMissingObject},
		{name: "lowercase key is not the Info object", body: `{"info":{}}`, want: ErrMissingObject},
		{name: "Info with wrong field type is malformed", body: `{"Info":{"radios":"x"}}`, want: ErrMalformedObject},
		{name: "Sensors with garbage temperature is malformed", body: `{"Sensors":{"esp32_temp":"hot"}}`, sensors: true, want: ErrMalformedObject},
		{name: "plain text has no Sensors object", body: "URL NOT FOUND", sensors: true, want: ErrMissingObject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.sensors {
				_, err = ParseSensors([]byte(tt.body))
			} else {
				_, err = ParseInfo([]byte(tt.body))
			}

			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestEnumNames checks pysmlight names and the numeric fallback for codes a
// newer firmware may add.
func TestEnumNames(t *testing.T) {
	tests := []struct {
		name   string
		lookup func(int64) string
		code   int64
		want   string
	}{
		{name: "zb_type 8 is otbr", lookup: ZBTypeName, code: 8, want: "otbr"},
		{name: "zb_type 5 is Zwave-EU", lookup: ZBTypeName, code: 5, want: "Zwave-EU"},
		{name: "zb_type 3 has no name and falls back to its code", lookup: ZBTypeName, code: 3, want: "3"},
		{name: "zb_channel -1 is unknown", lookup: ZBChannelName, code: -1, want: "unknown"},
		{name: "lte_state 5 is no_signal", lookup: LTEStateName, code: 5, want: "no_signal"},
		{name: "ble state 2 is started", lookup: BLEStateName, code: 2, want: "started"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.lookup(tt.code); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// assertRadios compares the label view of radios with want.
func assertRadios(t *testing.T, radios []Radio, want []wantRadio) {
	t.Helper()

	if len(radios) != len(want) {
		t.Fatalf("got %d radios, want %d", len(radios), len(want))
	}

	for i := range radios {
		r := &radios[i]

		got := wantRadio{
			index: r.Index(), hw: r.HW, version: r.FirmwareVersion(),
			typeName: r.TypeName(), channel: r.ChannelName(),
		}
		if got != want[i] {
			t.Errorf("radio %d = %+v, want %+v", i, got, want[i])
		}
	}
}

// readFixture loads a file from testdata.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	return data
}
