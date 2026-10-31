package slzbos

import "encoding/json"

// Addons lists optional hardware modules (lte, poe, zwave, mic) and whether
// each is fitted. A map rather than fixed fields keeps addons introduced by
// newer firmware visible without a code change.
type Addons map[string]bool

// Has reports whether the named addon is present and fitted.
func (a Addons) Has(name string) bool { return a[name] }

// UnmarshalJSON decodes the "addons" object, keeping only its boolean members.
//
// The object is not purely boolean on real hardware: SLZB-Ultima3 firmware
// v3.4.5.dev0 also sends "addon_type" as an integer. Decoding straight into a
// map[string]bool would reject that member and, with it, the whole /ha_info
// payload, so non-boolean members are skipped instead: they describe the
// addon set rather than one fitted module.
func (a *Addons) UnmarshalJSON(data []byte) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return err
	}

	// JSON null leaves the map nil; keep the receiver untouched, as
	// encoding/json does for a null map.
	if members == nil {
		return nil
	}

	addons := make(Addons, len(members))

	for name, raw := range members {
		var fitted bool
		if json.Unmarshal(raw, &fitted) == nil {
			addons[name] = fitted
		}
	}

	*a = addons

	return nil
}
