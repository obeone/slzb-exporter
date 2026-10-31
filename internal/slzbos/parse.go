package slzbos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrMissingObject reports a body that is not a JSON object holding the
// expected top-level key ("Info" or "Sensors"). Firmware without the
// endpoint answers with HTML or plain text, so this means "unsupported",
// not "broken".
var ErrMissingObject = errors.New("slzbos: expected JSON object not found")

// ErrMalformedObject reports an expected object that is present but cannot be
// decoded (wrong field types): the endpoint exists and misbehaves.
var ErrMalformedObject = errors.New("slzbos: malformed JSON object")

// infoKey and sensorsKey are the envelope keys of /ha_info and /ha_sensors.
const (
	infoKey    = "Info"
	sensorsKey = "Sensors"
)

// ParseInfo decodes a /ha_info body and applies normalisation. It returns an
// error wrapping ErrMissingObject when the "Info" object is absent and one
// wrapping ErrMalformedObject when it cannot be decoded.
func ParseInfo(body []byte) (*Info, error) {
	var info Info
	if err := decodeEnvelope(body, infoKey, &info); err != nil {
		return nil, err
	}

	normalizeInfo(&info)

	return &info, nil
}

// ParseSensors decodes a /ha_sensors body. Errors wrap ErrMissingObject or
// ErrMalformedObject like ParseInfo.
func ParseSensors(body []byte) (*Sensors, error) {
	var sensors Sensors
	if err := decodeEnvelope(body, sensorsKey, &sensors); err != nil {
		return nil, err
	}

	return &sensors, nil
}

// decodeEnvelope decodes body[key] into out.
//
// Decoding is two-step so that "no such object" (not JSON, wrong key, null)
// and "object present but broken" stay distinguishable: callers treat the
// first as an unsupported endpoint and the second as a real failure. The key
// lookup is exact, unlike encoding/json's case-insensitive struct matching.
func decodeEnvelope(body []byte, key string, out any) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrMissingObject, key, err)
	}

	raw, ok := envelope[key]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%w: %q", ErrMissingObject, key)
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrMalformedObject, key, err)
	}

	return nil
}
