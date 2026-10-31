package slzbos

import "strings"

// Normalisation applies pysmlight's semantic rules (Info.__post_init__ and
// check_zb_version) and nothing else: its cosmetic rewrites (model case,
// hw_version "1.04", sw_version suffix fixes) are skipped on purpose so that
// label values mirror what the firmware sent.

// ultimaRadioCount is the number of radios an Ultima has without the optional
// Z-Wave module.
const ultimaRadioCount = 2

// normalizeInfo applies every semantic rule to info, in pysmlight's order.
func normalizeInfo(info *Info) {
	truncateMissingZWave(info)
	synthesizeLegacyRadio(info)

	for i := range info.Radios {
		normalizeRadioVersion(&info.Radios[i])
	}
}

// truncateMissingZWave drops the third radio of an Ultima whose Z-Wave addon
// is not fitted: the firmware still reports the empty Z-Wave slot.
func truncateMissingZWave(info *Info) {
	if !strings.Contains(info.Model, "Ultima") || info.Addons.Has("zwave") {
		return
	}

	if len(info.Radios) > ultimaRadioCount {
		info.Radios = info.Radios[:ultimaRadioCount]
	}
}

// synthesizeLegacyRadio builds radio 0 from the top-level zb_* fields when the
// firmware sends no radios array (older single-radio firmware such as the
// SLZB-06 on core v2.5), so that callers always iterate Info.Radios.
//
// The FlexNumber pointers are shared with the top-level fields; this is safe
// because normalisation replaces pointers and never mutates a FlexNumber.
func synthesizeLegacyRadio(info *Info) {
	if len(info.Radios) > 0 {
		return
	}

	info.Radios = []Radio{{
		ChipIndex:   newFlexInt(0),
		HW:          info.ZBHW,
		Version:     info.ZBVersion,
		Type:        info.ZBType,
		Channel:     info.ZBChannel,
		RAMSizeKB:   info.ZBRAMSize,
		FlashSizeKB: info.ZBFlashSize,
	}}
}

// normalizeRadioVersion turns the zb_version sentinel -1 into "no version" and
// forces the channel to custom, as pysmlight does: a radio without a known
// catalogue firmware cannot be on the stable or dev channel.
func normalizeRadioVersion(radio *Radio) {
	version, ok := radio.Version.Int64()
	if !ok || version != zbVersionNone {
		return
	}

	radio.Version = nil
	radio.Channel = newFlexInt(ZBChannelCustom)
}
