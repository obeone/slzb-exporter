package collectors

import (
	"encoding/base64"
	"strings"
	"testing"
)

// These tests cover R9 end to end: a device protected by a web password, as
// the real SLZB-Ultima3 is, with and without credentials configured.

// testUsername is the web UI user of the password-protected fake device.
const testUsername = "admin"

// TestDeviceRequiringAuthWithoutCredentialsIsAuthError covers case j): every
// endpoint answers 401, which is counted as auth_error, logged once at warn
// with a configuration hint, and leaves the device unreachable.
func TestDeviceRequiringAuthWithoutCredentialsIsAuthError(t *testing.T) {
	logs := captureLogs(t)

	dev := newFakeDevice(t, slzb06CoreRoutes(t))
	dev.requireAuth(testUsername, testPassword)

	tc := newTestCollector(t, dev.URL(), collectorOptions{})

	tc.collect()

	assertValue(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", "0", "error_type", "auth_error"), 1)
	assertValue(t, tc.registry.SLZBHTTPErrorsTotal, tc.labels("action", actionHAInfo, "error_type", "auth_error"), 1)
	assertReachable(t, tc, 0)
	assertValue(t, tc.registry.SLZBCollectionErrors, tc.labels("error_type", "device_unreachable"), 1)

	if n := logs.countLines("level=WARN", "401"); n != 1 {
		t.Errorf("got %d auth warnings in one cycle, want exactly 1\n%s", n, logs)
	}

	if n := logs.countLines("level=WARN", "SLZB_EXPORTER_SLZB_PASSWORD"); n != 1 {
		t.Errorf("auth warning lacks the configuration hint\n%s", logs)
	}

	// Configuration is read from the environment only: a hint pointing at
	// YAML keys would send the operator to settings nothing reads.
	if n := logs.countLines("level=WARN", "yaml"); n != 0 {
		t.Errorf("auth warning points to yaml keys that are never loaded\n%s", logs)
	}

	if n := dev.requestCount(routeHASensors); n != 0 {
		t.Errorf("%s requested %d times after /ha_info failed, want 0", routeHASensors, n)
	}
}

// TestConfiguredCredentialsAreSentOnEveryRequest covers case k): with
// username and password configured, every request carries the Basic header,
// metrics from both sources are collected and the password is never logged.
func TestConfiguredCredentialsAreSentOnEveryRequest(t *testing.T) {
	logs := captureLogs(t)

	dev := newFakeDevice(t, slzb06CoreRoutes(t))
	dev.requireAuth(testUsername, testPassword)

	tc := newTestCollector(t, dev.URL(), collectorOptions{username: testUsername, password: testPassword})

	tc.collect()

	assertEveryRequestAuthenticated(t, dev, []string{routeLegacy, routeHAInfo, routeHASensors, routeConfig})

	assertReachable(t, tc, 1)
	assertValue(t, tc.registry.SLZBDeviceTemp, tc.labels(), legacyTemp)
	assertSeriesCount(t, tc.registry.OS.DeviceInfo, 1)
	assertSeriesCount(t, tc.registry.SLZBHTTPErrorsTotal, 0)

	if strings.Contains(logs.String(), testPassword) {
		t.Error("the password appears in the logs")
	}
}

// assertEveryRequestAuthenticated checks that the device received exactly one
// request per expected route and that each carried the expected Basic header.
func assertEveryRequestAuthenticated(t *testing.T, dev *fakeDevice, routes []string) {
	t.Helper()

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(testUsername+":"+testPassword))

	requests := dev.requests()
	if len(requests) != len(routes) {
		t.Errorf("device got %d requests, want %d (one per route %v)", len(requests), len(routes), routes)
	}

	for _, req := range requests {
		if req.authorization != want {
			t.Errorf("request to %s has an unexpected Authorization header", req.route)
		}
	}

	for _, route := range routes {
		if n := dev.requestCount(route); n != 1 {
			t.Errorf("%s requested %d times, want 1", route, n)
		}
	}
}
