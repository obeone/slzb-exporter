package collectors

import (
	"net/http"
	"testing"
)

// TestClassifyLegacyResponse checks how every kind of legacy /api?action=0
// answer maps to the R3 tri-state.
func TestClassifyLegacyResponse(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		hasValues bool
		body      string
		want      legacyStatus
	}{
		{name: "200 with respValuesArr is ok", status: http.StatusOK, hasValues: true, want: legacyOK},
		{name: "200 without header is absent", status: http.StatusOK, want: legacyAbsent},
		{name: "200 without header and URL NOT FOUND body is absent", status: http.StatusOK, body: "URL NOT FOUND", want: legacyAbsent},
		{name: "404 is absent", status: http.StatusNotFound, body: "Not found", want: legacyAbsent},
		{name: "500 with URL NOT FOUND body is absent", status: http.StatusInternalServerError, body: " URL NOT FOUND\n", want: legacyAbsent},
		{name: "500 is failed", status: http.StatusInternalServerError, body: "oops", want: legacyFailed},
		{name: "401 is failed", status: http.StatusUnauthorized, body: "wrong login or password", want: legacyFailed},
		{name: "503 with empty body is failed", status: http.StatusServiceUnavailable, want: legacyFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyLegacyResponse(tt.status, tt.hasValues, []byte(tt.body))
			if got != tt.want {
				t.Errorf("classifyLegacyResponse(%d, %v, %q) = %s, want %s", tt.status, tt.hasValues, tt.body, got, tt.want)
			}
		})
	}
}

// TestLegacyResultTransportFailed checks that only a failure without any
// response skips the /ha_* step.
func TestLegacyResultTransportFailed(t *testing.T) {
	if !(legacyResult{status: legacyFailed}).transportFailed() {
		t.Error("failed without response should be a transport failure")
	}

	if (legacyResult{status: legacyFailed, httpStatus: http.StatusInternalServerError}).transportFailed() {
		t.Error("failed with a 500 response should not be a transport failure")
	}

	if (legacyResult{status: legacyAbsent, httpStatus: http.StatusNotFound}).transportFailed() {
		t.Error("absent should not be a transport failure")
	}
}

// TestLegacyResultReachable checks the R3 reachability rule, both for
// devices serving /ha_* and for legacy-only devices.
func TestLegacyResultReachable(t *testing.T) {
	tests := []struct {
		name        string
		result      legacyResult
		haSupported bool
		haOK        bool
		want        bool
	}{
		{name: "legacy ok alone", result: legacyResult{legacyOK, 200}, want: true},
		{name: "legacy-only 200 without header keeps defaults behaviour", result: legacyResult{legacyAbsent, 200}, want: true},
		{name: "legacy-only 404", result: legacyResult{legacyAbsent, 404}, want: false},
		{name: "legacy-only malformed header", result: legacyResult{legacyFailed, 200}, want: false},
		{name: "legacy-only transport error", result: legacyResult{legacyFailed, 0}, want: false},
		{name: "ha ok with legacy absent", result: legacyResult{legacyAbsent, 404}, haSupported: true, haOK: true, want: true},
		{name: "ha ok with legacy 500", result: legacyResult{legacyFailed, 500}, haSupported: true, haOK: true, want: true},
		{name: "ha failed with legacy ok", result: legacyResult{legacyOK, 200}, haSupported: true, want: true},
		{name: "ha failed with legacy 200 without header", result: legacyResult{legacyAbsent, 200}, haSupported: true, want: false},
		{name: "ha failed with legacy absent", result: legacyResult{legacyAbsent, 404}, haSupported: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.reachable(tt.haSupported, tt.haOK); got != tt.want {
				t.Errorf("reachable(%v, %v) = %v, want %v", tt.haSupported, tt.haOK, got, tt.want)
			}
		})
	}
}

// TestLegacyResultCountsHTTPError checks that an absent legacy endpoint is
// quiet only when /ha_* is supported, and real failures are always counted.
func TestLegacyResultCountsHTTPError(t *testing.T) {
	tests := []struct {
		name        string
		result      legacyResult
		haSupported bool
		want        bool
	}{
		{name: "404 on legacy-only device is counted", result: legacyResult{legacyAbsent, 404}, want: true},
		{name: "404 on SLZB-OS device is quiet", result: legacyResult{legacyAbsent, 404}, haSupported: true, want: false},
		{name: "URL NOT FOUND 500 on SLZB-OS device is quiet", result: legacyResult{legacyAbsent, 500}, haSupported: true, want: false},
		{name: "500 on SLZB-OS device is counted", result: legacyResult{legacyFailed, 500}, haSupported: true, want: true},
		{name: "401 is counted", result: legacyResult{legacyFailed, 401}, haSupported: true, want: true},
		{name: "200 without header is never an HTTP error", result: legacyResult{legacyAbsent, 200}, want: false},
		{name: "transport error is counted elsewhere", result: legacyResult{legacyFailed, 0}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.countsHTTPError(tt.haSupported); got != tt.want {
				t.Errorf("countsHTTPError(%v) = %v, want %v", tt.haSupported, got, tt.want)
			}
		})
	}
}

// TestLegacyResultRunsConfigStep checks when /api?action=4 is requested.
func TestLegacyResultRunsConfigStep(t *testing.T) {
	tests := []struct {
		name        string
		result      legacyResult
		haSupported bool
		reachable   bool
		want        bool
	}{
		{name: "legacy ok", result: legacyResult{legacyOK, 200}, haSupported: true, reachable: true, want: true},
		{name: "legacy-only reachable through defaults", result: legacyResult{legacyAbsent, 200}, reachable: true, want: true},
		{name: "SLZB-OS device without legacy data", result: legacyResult{legacyAbsent, 404}, haSupported: true, reachable: true, want: false},
		{name: "unreachable legacy-only device", result: legacyResult{legacyFailed, 500}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.runsConfigStep(tt.haSupported, tt.reachable); got != tt.want {
				t.Errorf("runsConfigStep(%v, %v) = %v, want %v", tt.haSupported, tt.reachable, got, tt.want)
			}
		})
	}
}
