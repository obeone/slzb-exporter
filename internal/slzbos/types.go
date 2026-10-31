// Package slzbos reads the SLZB-OS Home Assistant endpoints (/ha_info and
// /ha_sensors) served by SMLIGHT SLZB devices running core firmware 2.x and
// later, including the multi-radio SLZB-Ultima range.
//
// Field names and semantics follow pysmlight, SMLIGHT's reference client. The
// package only fetches, decodes and normalises; it has no Prometheus
// dependency so that the metric mapping stays in the collectors package.
package slzbos

// Optional scalars are pointers (or nil-safe *FlexNumber) because firmware
// versions and models differ in which keys they send: a nil field means the
// key was absent, which callers must not confuse with a zero value.

// Info is the "Info" object returned by GET /ha_info, after normalisation.
type Info struct {
	Model     string      `json:"model"`
	Hostname  string      `json:"hostname"`
	SWVersion string      `json:"sw_version"`
	HWVersion *FlexNumber `json:"hw_version"`
	MAC       string      `json:"MAC"`
	// FWChannel is the device-level firmware channel string ("dev", "release").
	FWChannel string      `json:"fw_channel"`
	LegacyAPI *FlexNumber `json:"legacy_api"`

	// Memory and filesystem sizes, in kilobytes as sent by the firmware.
	RAMTotalKB   *FlexNumber `json:"ram_total"`
	PSRAMTotalKB *FlexNumber `json:"psram_total"`
	FSTotalKB    *FlexNumber `json:"fs_total"`

	Radios []Radio      `json:"radios"`
	Addons Addons       `json:"addons"`
	BLE    *BLEFeatures `json:"ble"`

	// Top-level radio fields sent by single-radio firmware (SLZB-06 style).
	// They only feed the synthesised radio 0 when Radios is empty.
	ZBHW        string      `json:"zb_hw"`
	ZBVersion   *FlexNumber `json:"zb_version"`
	ZBType      *FlexNumber `json:"zb_type"`
	ZBChannel   *FlexNumber `json:"zb_channel"`
	ZBRAMSize   *FlexNumber `json:"zb_ram_size"`
	ZBFlashSize *FlexNumber `json:"zb_flash_size"`
}

// Radio describes one radio chip of the device (Zigbee, Thread or Z-Wave).
type Radio struct {
	ChipIndex   *FlexNumber `json:"chip_index"`
	HW          string      `json:"zb_hw"`
	Version     *FlexNumber `json:"zb_version"`
	Type        *FlexNumber `json:"zb_type"`
	Channel     *FlexNumber `json:"zb_channel"`
	RAMSizeKB   *FlexNumber `json:"zb_ram_size"`
	FlashSizeKB *FlexNumber `json:"zb_flash_size"`
}

// Index returns the chip index as label text ("0", "1", ...), "" when unset.
func (r *Radio) Index() string { return r.ChipIndex.String() }

// FirmwareVersion returns the radio firmware version as raw label text. It is
// "" when the firmware reports no version (-1, cleared by normalisation).
func (r *Radio) FirmwareVersion() string { return r.Version.String() }

// TypeName returns the radio role name (e.g. "coordinator", "otbr").
func (r *Radio) TypeName() string { return flexEnumName(r.Type, ZBTypeName) }

// ChannelName returns the radio firmware channel name (e.g. "stable", "custom").
func (r *Radio) ChannelName() string { return flexEnumName(r.Channel, ZBChannelName) }

// BLEFeatures is the "ble" object of /ha_info: configured BLE features.
type BLEFeatures struct {
	Enabled      *bool `json:"ble_enabled"`
	ProxyEnabled *bool `json:"proxy_enabled"`
}

// Sensors is the "Sensors" object returned by GET /ha_sensors. Ambilight,
// LED and other toggle settings are deliberately not decoded.
type Sensors struct {
	// Temperatures in degrees Celsius, usually sent as strings.
	ESP32Temp *FlexNumber `json:"esp32_temp"`
	ZBTemp    *FlexNumber `json:"zb_temp"`
	ZBTemp2   *FlexNumber `json:"zb_temp2"`

	// Uptimes in seconds; 0 means "no client connected" for sockets and OTBR.
	Uptime        *FlexNumber `json:"uptime"`
	SocketUptime  *FlexNumber `json:"socket_uptime"`
	Socket2Uptime *FlexNumber `json:"socket2_uptime"`
	Socket3Uptime *FlexNumber `json:"socket3_uptime"`
	OTBRUptime    *FlexNumber `json:"otbr_uptime"`

	// Memory and filesystem usage, in kilobytes.
	RAMUsageKB   *FlexNumber `json:"ram_usage"`
	PSRAMUsageKB *FlexNumber `json:"psram_usage"`
	FSUsedKB     *FlexNumber `json:"fs_used"`

	// Largest contiguous free block of RAM and PSRAM, in kilobytes: a heap
	// fragmentation signal. Seen on SLZB-Ultima3 v3.4.5.dev0, unknown to
	// pysmlight, so most devices leave them nil.
	RAMLargestFreeBlockKB   *FlexNumber `json:"ram_largest_free_block"`
	PSRAMLargestFreeBlockKB *FlexNumber `json:"psram_largest_free_block"`

	Ethernet      *bool `json:"ethernet"`
	WifiConnected *bool `json:"wifi_connected"`
	VPNStatus     *bool `json:"vpn_status"`
	VPNEnabled    *bool `json:"vpn_enabled"`

	LTEDetect *bool       `json:"lte_detect"`
	LTEState  *FlexNumber `json:"lte_state"`

	BLE *BLESession `json:"ble"`
}

// BLESession is the "ble" object of /ha_sensors: live BLE stack state.
type BLESession struct {
	State          *FlexNumber `json:"state"`
	ProxyConnected *bool       `json:"proxy_connected"`
}
