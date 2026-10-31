package collectors

import (
	"bytes"
	"net/http"
)

// legacyStatus is the tri-state outcome of the legacy GET /api?action=0 step.
//
// A plain bool is not enough since SLZB-OS v3 devices (SLZB-Ultima3) may not
// serve the legacy endpoint at all: "absent" must stay quiet when the
// /ha_* endpoints answer, while "failed" is always a real error.
type legacyStatus int

const (
	// legacyOK means the respValuesArr header was present and parsed.
	legacyOK legacyStatus = iota
	// legacyAbsent means the firmware does not serve the legacy data: HTTP
	// 404, a "URL NOT FOUND" body, or a 200 without the respValuesArr header.
	legacyAbsent
	// legacyFailed means a transport error, another non-200 status or a
	// malformed respValuesArr header.
	legacyFailed
)

// legacyURLNotFoundBody is what SMLIGHT firmware answers for an unknown route,
// sometimes with a non-404 status. It mirrors the unexported constant of the
// slzbos package, which applies the same rule to /ha_*.
const legacyURLNotFoundBody = "URL NOT FOUND"

// legacyBodyPeekBytes caps how much of a non-200 legacy body is read: only a
// short "URL NOT FOUND" marker is looked for, never the full payload.
const legacyBodyPeekBytes = 256

// String returns a lowercase name suitable for logs and span attributes.
func (s legacyStatus) String() string {
	switch s {
	case legacyOK:
		return "ok"
	case legacyAbsent:
		return "absent"
	case legacyFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// legacyResult is what the legacy step hands to collectMetrics, which decides
// reachability and error accounting only after the /ha_* step has run.
type legacyResult struct {
	status legacyStatus
	// httpStatus is the response status code, 0 when no response arrived
	// (transport error).
	httpStatus int
}

// classifyLegacyResponse maps a legacy /api?action=0 response to its status.
//
// hasValues reports whether the respValuesArr header is present; body is the
// start of the response body (only needed for non-200 answers). A 200 with the
// header is classified ok here; a later parse failure downgrades it to failed.
func classifyLegacyResponse(statusCode int, hasValues bool, body []byte) legacyStatus {
	switch {
	case statusCode == http.StatusOK && hasValues:
		return legacyOK
	case statusCode == http.StatusOK, statusCode == http.StatusNotFound:
		return legacyAbsent
	case string(bytes.TrimSpace(body)) == legacyURLNotFoundBody:
		return legacyAbsent
	default:
		return legacyFailed
	}
}

// transportFailed reports whether the legacy request got no response at all.
// The /ha_* step is then skipped so a dead device does not stack timeouts.
func (r legacyResult) transportFailed() bool {
	return r.status == legacyFailed && r.httpStatus == 0
}

// authFailed reports whether the device rejected the legacy request with 401.
func (r legacyResult) authFailed() bool {
	return r.httpStatus == http.StatusUnauthorized
}

// reachableAlone is the pre-SLZB-OS reachability rule, kept unchanged for
// devices without /ha_*: parsed data, or a 200 without the header (which the
// collector has always treated as reachable with default device info).
func (r legacyResult) reachableAlone() bool {
	return r.status == legacyOK || (r.status == legacyAbsent && r.httpStatus == http.StatusOK)
}

// reachable decides slzb_device_reachable from both steps. When /ha_* is not
// supported the legacy-only rule applies, so legacy-only devices keep their
// exact historical behaviour.
func (r legacyResult) reachable(haSupported, haOK bool) bool {
	if !haSupported {
		return r.reachableAlone()
	}

	return r.status == legacyOK || haOK
}

// countsHTTPError reports whether a non-200 legacy answer must be counted in
// slzb_http_errors_total{action="0"}. An absent legacy endpoint is expected,
// hence quiet, on firmware that serves /ha_*.
func (r legacyResult) countsHTTPError(haSupported bool) bool {
	if r.httpStatus == 0 || r.httpStatus == http.StatusOK {
		return false
	}

	return r.status != legacyAbsent || !haSupported
}

// runsConfigStep reports whether /api?action=4 is worth requesting: with
// legacy data, or on a legacy-only device that is reachable (historical
// behaviour). A /ha_* device without legacy data would only answer errors.
func (r legacyResult) runsConfigStep(haSupported, reachable bool) bool {
	return r.status == legacyOK || (!haSupported && reachable)
}
