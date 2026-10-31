package slzbos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeResponse is what the fake device answers on every path.
type fakeResponse struct {
	status int
	body   string
}

// newFakeDevice starts an httptest server answering resp on every request.
func newFakeDevice(t *testing.T, resp fakeResponse) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
	}))
	t.Cleanup(srv.Close)

	return srv
}

// TestFetchInfoOutcomes checks how every kind of /ha_info answer is classified.
func TestFetchInfoOutcomes(t *testing.T) {
	ultima := string(readFixture(t, "ha_info_ultima3.json"))

	tests := []struct {
		name       string
		resp       fakeResponse
		wantStatus Status
		wantAuth   bool
		wantInfo   bool
	}{
		{name: "200 with Info object is OK", resp: fakeResponse{http.StatusOK, ultima}, wantStatus: StatusOK, wantInfo: true},
		{name: "404 is unsupported", resp: fakeResponse{http.StatusNotFound, "Not found"}, wantStatus: StatusUnsupported},
		{name: "200 with URL NOT FOUND body is unsupported", resp: fakeResponse{http.StatusOK, "URL NOT FOUND"}, wantStatus: StatusUnsupported},
		{name: "500 with URL NOT FOUND body is unsupported", resp: fakeResponse{http.StatusInternalServerError, "URL NOT FOUND\n"}, wantStatus: StatusUnsupported},
		{name: "200 with HTML page is unsupported", resp: fakeResponse{http.StatusOK, "<!DOCTYPE html><html></html>"}, wantStatus: StatusUnsupported},
		{name: "200 with malformed Info object fails", resp: fakeResponse{http.StatusOK, `{"Info":{"radios":1}}`}, wantStatus: StatusFailed},
		{name: "500 fails", resp: fakeResponse{http.StatusInternalServerError, "oops"}, wantStatus: StatusFailed},
		{name: "401 fails and is flagged as auth error", resp: fakeResponse{http.StatusUnauthorized, "wrong login or password"}, wantStatus: StatusFailed, wantAuth: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeDevice(t, tt.resp)

			info, outcome := NewClient(srv.Client(), srv.URL+"/").FetchInfo(context.Background())

			assertOutcome(t, outcome, tt.wantStatus, tt.resp.status, tt.wantAuth)

			if (info != nil) != tt.wantInfo {
				t.Errorf("info returned = %v, want %v", info != nil, tt.wantInfo)
			}
		})
	}
}

// TestFetchSensorsOK checks the happy path of /ha_sensors.
func TestFetchSensorsOK(t *testing.T) {
	srv := newFakeDevice(t, fakeResponse{http.StatusOK, string(readFixture(t, "ha_sensors_ultima3.json"))})

	sensors, outcome := NewClient(srv.Client(), srv.URL).FetchSensors(context.Background())

	assertOutcome(t, outcome, StatusOK, http.StatusOK, false)

	if sensors == nil || sensors.ESP32Temp.Float64() != 39.44 {
		t.Errorf("sensors not decoded: %+v", sensors)
	}
}

// TestFetchTransportErrorFails checks that an unreachable device is a failure
// with no HTTP status, never mistaken for an unsupported endpoint.
func TestFetchTransportErrorFails(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	info, outcome := NewClient(&http.Client{}, url).FetchInfo(context.Background())

	assertOutcome(t, outcome, StatusFailed, 0, false)

	if info != nil {
		t.Error("no Info expected on transport error")
	}
}

// assertOutcome checks status, HTTP code and auth flag, and that every
// non-OK outcome carries a reason.
func assertOutcome(t *testing.T, got Outcome, wantStatus Status, wantHTTP int, wantAuth bool) {
	t.Helper()

	if got.Status != wantStatus {
		t.Errorf("Status = %s, want %s (err: %v)", got.Status, wantStatus, got.Err)
	}

	if got.HTTPStatus != wantHTTP {
		t.Errorf("HTTPStatus = %d, want %d", got.HTTPStatus, wantHTTP)
	}

	if got.AuthFailed() != wantAuth {
		t.Errorf("AuthFailed() = %v, want %v", got.AuthFailed(), wantAuth)
	}

	if (got.Err != nil) != (wantStatus != StatusOK) {
		t.Errorf("Err = %v, want error only for non-OK outcomes", got.Err)
	}
}
