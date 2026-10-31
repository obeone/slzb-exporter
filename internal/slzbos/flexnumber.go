package slzbos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FlexNumber is a numeric value that SLZB-OS may send either as a JSON number
// (`104`) or as a numeric JSON string (`"39.44"`).
//
// The raw token text is kept verbatim so that values used as Prometheus label
// values (hardware revision, radio firmware version) are rendered exactly as
// the firmware sent them: formatting 20251218 through a float would produce
// "2.0251218e+07". The parsed float is kept alongside for gauge values.
//
// A nil *FlexNumber, a JSON null and an empty string all mean "not set"; every
// method is nil-safe so callers can query optional fields directly.
type FlexNumber struct {
	raw   string
	value float64
}

// newFlexInt builds a set FlexNumber from an integer, used when normalisation
// has to substitute a value the firmware did not send.
func newFlexInt(v int64) *FlexNumber {
	return &FlexNumber{raw: strconv.FormatInt(v, 10), value: float64(v)}
}

// UnmarshalJSON decodes a JSON number or a numeric JSON string.
//
// A string must hold a valid JSON number once surrounding whitespace is
// trimmed; anything else is an error so that garbage never turns into a
// silent zero. JSON null and "" leave the value unset.
func (n *FlexNumber) UnmarshalJSON(data []byte) error {
	text, err := flexNumberText(data)
	if err != nil {
		return err
	}

	if text == "" {
		*n = FlexNumber{}

		return nil
	}

	value, err := parseJSONNumber(text)
	if err != nil {
		return err
	}

	*n = FlexNumber{raw: text, value: value}

	return nil
}

// flexNumberText extracts the candidate number text from a raw JSON token,
// unquoting strings. It returns "" for null and for empty or blank strings.
func flexNumberText(data []byte) (string, error) {
	token := bytes.TrimSpace(data)
	if bytes.Equal(token, []byte("null")) {
		return "", nil
	}

	if len(token) == 0 || token[0] != '"' {
		return string(token), nil
	}

	var s string
	if err := json.Unmarshal(token, &s); err != nil {
		return "", fmt.Errorf("slzbos: invalid numeric string %s: %w", token, err)
	}

	return strings.TrimSpace(s), nil
}

// parseJSONNumber validates text against the JSON number grammar and returns
// its float value. Decoding through encoding/json (rather than
// strconv.ParseFloat) rejects Go-only spellings such as "NaN", "Inf" or hex
// floats, which a firmware would never mean as a number. Decoding into a
// float64 (not a json.Number) also rejects a doubly quoted string.
func parseJSONNumber(text string) (float64, error) {
	var value float64
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return 0, fmt.Errorf("slzbos: %q is not a number: %w", text, err)
	}

	return value, nil
}

// IsSet reports whether the firmware sent a numeric value.
func (n *FlexNumber) IsSet() bool {
	return n != nil && n.raw != ""
}

// String returns the raw token text as sent by the firmware, or "" when unset.
func (n *FlexNumber) String() string {
	if n == nil {
		return ""
	}

	return n.raw
}

// Float64 returns the numeric value, or 0 when unset. Callers that must tell
// a real zero from a missing key check IsSet first.
func (n *FlexNumber) Float64() float64 {
	if n == nil {
		return 0
	}

	return n.value
}

// Int64 returns the value as an integer and true when it is set and integral
// (enum codes, sizes). It returns false for unset or fractional values.
func (n *FlexNumber) Int64() (int64, bool) {
	if !n.IsSet() || n.value != math.Trunc(n.value) {
		return 0, false
	}

	if n.value < math.MinInt64 || n.value >= math.MaxInt64 {
		return 0, false
	}

	return int64(n.value), true
}
