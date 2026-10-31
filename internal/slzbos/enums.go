package slzbos

import "strconv"

// Enum codes and names mirror pysmlight's const.py (ZB_TYPES, ZB_CHANNEL,
// PppUSBState, BleState), the reference client maintained by SMLIGHT.

// ZBChannelCustom is the zb_channel code for a custom (non-catalogue) radio
// firmware; normalisation forces it when zb_version is -1.
const ZBChannelCustom = 2

// zbVersionNone is the zb_version sentinel meaning "no known firmware".
const zbVersionNone = -1

// zbTypeNames maps zb_type codes to the radio role names used by pysmlight.
var zbTypeNames = map[int64]string{
	-1: "unknown",
	0:  "coordinator",
	1:  "router",
	2:  "thread",
	4:  "zhub",
	5:  "Zwave-EU",
	6:  "Zwave-US",
	7:  "Zwave-ANZ",
	8:  "otbr",
}

// zbChannelNames maps zb_channel codes to firmware release channel names.
var zbChannelNames = map[int64]string{
	-1: "unknown",
	0:  "stable",
	1:  "dev",
	2:  "custom",
}

// lteStateNames maps Sensors.lte_state codes (PppUSBState) to names.
var lteStateNames = map[int64]string{
	0: "idle",
	1: "starting",
	2: "connecting",
	3: "connected",
	4: "error",
	5: "no_signal",
}

// bleStateNames maps Sensors.ble.state codes (BleState) to names.
var bleStateNames = map[int64]string{
	0: "idle",
	1: "starting",
	2: "started",
	3: "error",
	4: "stopping",
	5: "stopped",
}

// ZBTypeName returns the radio role name for a zb_type code.
func ZBTypeName(code int64) string { return enumName(zbTypeNames, code) }

// ZBChannelName returns the release channel name for a zb_channel code.
func ZBChannelName(code int64) string { return enumName(zbChannelNames, code) }

// LTEStateName returns the LTE modem state name for an lte_state code.
func LTEStateName(code int64) string { return enumName(lteStateNames, code) }

// BLEStateName returns the BLE stack state name for a ble.state code.
func BLEStateName(code int64) string { return enumName(bleStateNames, code) }

// enumName looks code up in names. An unknown code falls back to its decimal
// text so that a firmware adding a new value still yields a distinct,
// meaningful label instead of collapsing into a shared "unknown".
func enumName(names map[int64]string, code int64) string {
	if name, ok := names[code]; ok {
		return name
	}

	return strconv.FormatInt(code, 10)
}

// flexEnumName applies lookup to an optional FlexNumber. It returns "" when
// the value is unset and the raw text when it is not an integer.
func flexEnumName(n *FlexNumber, lookup func(int64) string) string {
	if !n.IsSet() {
		return ""
	}

	code, ok := n.Int64()
	if !ok {
		return n.String()
	}

	return lookup(code)
}
