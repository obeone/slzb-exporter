package slzbos

import (
	"encoding/json"
	"testing"
)

// TestFlexNumberDecoding checks that numbers and numeric strings decode with
// their raw text kept for labels, and that garbage is rejected.
func TestFlexNumberDecoding(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantSet   bool
		wantText  string
		wantFloat float64
	}{
		{name: "JSON number keeps raw text", input: `104`, wantSet: true, wantText: "104", wantFloat: 104},
		{name: "numeric string decodes to its value", input: `"39.44"`, wantSet: true, wantText: "39.44", wantFloat: 39.44},
		{name: "numeric string with spaces is trimmed", input: `" 12 "`, wantSet: true, wantText: "12", wantFloat: 12},
		{name: "minus one sentinel is a regular number", input: `-1`, wantSet: true, wantText: "-1", wantFloat: -1},
		{name: "minus one as string is a regular number", input: `"-1"`, wantSet: true, wantText: "-1", wantFloat: -1},
		{name: "large integer renders without float formatting", input: `20251218`, wantSet: true, wantText: "20251218", wantFloat: 20251218},
		{name: "null leaves value unset", input: `null`},
		{name: "empty string leaves value unset", input: `""`},
		{name: "non numeric string is an error", input: `"hot"`, wantErr: true},
		{name: "NaN string is an error", input: `"NaN"`, wantErr: true},
		{name: "boolean is an error", input: `true`, wantErr: true},
		{name: "object is an error", input: `{}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n FlexNumber

			err := json.Unmarshal([]byte(tt.input), &n)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %s, got value %q", tt.input, n.String())
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertFlex(t, &n, tt.wantSet, tt.wantText, tt.wantFloat)
		})
	}
}

// TestFlexNumberNilIsUnset checks that a nil pointer, the absent-key case, is
// safe to query and reads as unset.
func TestFlexNumberNilIsUnset(t *testing.T) {
	var n *FlexNumber

	assertFlex(t, n, false, "", 0)

	if _, ok := n.Int64(); ok {
		t.Error("Int64 on nil must report false")
	}
}

// TestFlexNumberInt64 checks integral detection used for enum lookups.
func TestFlexNumberInt64(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   int64
		wantOK bool
	}{
		{name: "integer number", input: `8`, want: 8, wantOK: true},
		{name: "integer string", input: `"3"`, want: 3, wantOK: true},
		{name: "negative integer", input: `-1`, want: -1, wantOK: true},
		{name: "fractional value is not an integer", input: `"39.44"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n FlexNumber
			if err := json.Unmarshal([]byte(tt.input), &n); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, ok := n.Int64()
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("Int64() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// assertFlex checks the set flag, raw text and float value of n.
func assertFlex(t *testing.T, n *FlexNumber, wantSet bool, wantText string, wantFloat float64) {
	t.Helper()

	if n.IsSet() != wantSet {
		t.Errorf("IsSet() = %v, want %v", n.IsSet(), wantSet)
	}

	if n.String() != wantText {
		t.Errorf("String() = %q, want %q", n.String(), wantText)
	}

	if n.Float64() != wantFloat {
		t.Errorf("Float64() = %v, want %v", n.Float64(), wantFloat)
	}
}
