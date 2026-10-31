package collectors

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/d0ugal/promexporter/tracing"
	"github.com/d0ugal/slzb-exporter/internal/slzbos"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
)

// Action label values used for the SLZB-OS endpoints in the request, latency
// and error metrics, next to the legacy "0", "4" and "firmware".
const (
	actionHAInfo    = "ha_info"
	actionHASensors = "ha_sensors"
)

// bytesPerKB converts the kilobyte sizes sent by SLZB-OS to bytes.
const bytesPerKB = 1024

// slzbOSResult summarises the /ha_* step for collectMetrics.
type slzbOSResult struct {
	// supported is false when /ha_info is unsupported or was not requested.
	supported bool
	// infoOK is true when /ha_info was fetched and parsed this cycle.
	infoOK bool
	// authFailed is true when any /ha_* endpoint answered 401.
	authFailed bool
}

// collectSLZBOSMetrics runs the SLZB-OS step: GET /ha_info, then /ha_sensors
// when the former parsed. It is skipped after a legacy transport error so a
// dead device does not cost one more client timeout per endpoint.
//
// The parsed Info is kept in sc.osInfo for this cycle only, as the firmware
// step reads its sw_version.
//
//nolint:contextcheck // Context is updated to include span for child operations
func (sc *SLZBCollector) collectSLZBOSMetrics(ctx context.Context, deviceName string, legacy legacyResult) slzbOSResult {
	sc.osInfo = nil

	if legacy.transportFailed() {
		slog.Debug("Skipping SLZB-OS endpoints after legacy transport error", "device", deviceName)
		return slzbOSResult{}
	}

	var collectorSpan *tracing.CollectorSpan

	if tracer := sc.app.GetTracer(); tracer != nil && tracer.IsEnabled() {
		collectorSpan = tracer.NewCollectorSpan(ctx, "slzb-collector", "collect-slzb-os")
		collectorSpan.SetAttributes(
			attribute.String("device.id", deviceName),
			attribute.String("device.action", actionHAInfo),
		)

		ctx = collectorSpan.Context()
		defer collectorSpan.End()
	}

	result := sc.collectSLZBOSInfo(ctx, collectorSpan, deviceName)
	if !result.infoOK {
		return result
	}

	if sc.collectSLZBOSSensors(ctx, collectorSpan, deviceName, legacy.status == legacyOK) {
		result.authFailed = true
	}

	return result
}

// collectSLZBOSInfo fetches /ha_info and applies its outcome: write the
// /ha_info gauges on success, clear every SLZB-OS series when the endpoint is
// unsupported, count the error otherwise (keeping the last values).
func (sc *SLZBCollector) collectSLZBOSInfo(ctx context.Context, span *tracing.CollectorSpan, deviceName string) slzbOSResult {
	start := time.Now()
	info, outcome := sc.osClient.FetchInfo(ctx)
	sc.recordSLZBOSRequest(span, deviceName, actionHAInfo, outcome, time.Since(start).Seconds())

	switch outcome.Status {
	case slzbos.StatusOK:
		sc.osInfo = info
		sc.writeSLZBOSInfo(deviceName, info)

		return slzbOSResult{supported: true, infoOK: true}
	case slzbos.StatusUnsupported:
		slog.Debug("SLZB-OS /ha_info not supported by device", "device", deviceName, "reason", outcome.Err)
		sc.metrics.OS.ResetAll(deviceName)

		return slzbOSResult{}
	default:
		return slzbOSResult{supported: true, authFailed: outcome.AuthFailed()}
	}
}

// collectSLZBOSSensors fetches /ha_sensors and writes its gauges. legacyOK
// tells whether the legacy step already fed the shared temperature and uptime
// gauges this cycle. It returns true when the device answered 401.
func (sc *SLZBCollector) collectSLZBOSSensors(ctx context.Context, span *tracing.CollectorSpan, deviceName string, legacyOK bool) bool {
	start := time.Now()
	sensors, outcome := sc.osClient.FetchSensors(ctx)
	sc.recordSLZBOSRequest(span, deviceName, actionHASensors, outcome, time.Since(start).Seconds())

	switch outcome.Status {
	case slzbos.StatusOK:
		sc.writeSLZBOSSensors(deviceName, sensors, legacyOK)
	case slzbos.StatusUnsupported:
		// /ha_info answered, so this is a partial firmware: treat the facts as
		// vanished rather than as an error.
		slog.Debug("SLZB-OS /ha_sensors not supported by device", "device", deviceName, "reason", outcome.Err)
		sc.metrics.OS.ResetSensors(deviceName)
	}

	return outcome.AuthFailed()
}

// recordSLZBOSRequest updates the request, latency and error metrics for one
// /ha_* fetch, the same way collectDeviceInfo does for the legacy endpoint.
// An unsupported endpoint is a normal answer: counted as a request, not as an
// error.
func (sc *SLZBCollector) recordSLZBOSRequest(span *tracing.CollectorSpan, deviceName, action string, outcome slzbos.Outcome, duration float64) {
	if outcome.HTTPStatus != 0 {
		sc.metrics.SLZBAPIResponseTimeSeconds.With(prometheus.Labels{
			"device": deviceName,
			"action": action,
		}).Observe(duration)
		sc.metrics.SLZBHTTPRequestsTotal.With(prometheus.Labels{
			"device": deviceName,
			"action": action,
			"status": strconv.Itoa(outcome.HTTPStatus),
		}).Inc()
	}

	if span != nil {
		span.AddEvent(action+"_fetched",
			attribute.String("outcome", outcome.Status.String()),
			attribute.Int("http.status_code", outcome.HTTPStatus),
			attribute.Float64("duration_seconds", duration),
		)
	}

	if outcome.Status == slzbos.StatusFailed {
		sc.recordSLZBOSFailure(span, deviceName, action, outcome)
	}
}

// recordSLZBOSFailure counts and logs a failed /ha_* fetch. A 401 is only
// counted here: collectMetrics logs a single auth warning per cycle.
func (sc *SLZBCollector) recordSLZBOSFailure(span *tracing.CollectorSpan, deviceName, action string, outcome slzbos.Outcome) {
	errorType := slzbOSErrorType(outcome)

	sc.metrics.SLZBHTTPErrorsTotal.With(prometheus.Labels{
		"device":     deviceName,
		"action":     action,
		"error_type": errorType,
	}).Inc()

	if errorType == "request_error" {
		sc.metrics.SLZBAPITimeoutErrorsTotal.With(prometheus.Labels{
			"device": deviceName,
			"action": action,
		}).Inc()
	}

	if span != nil {
		span.RecordError(outcome.Err, attribute.String("operation", action))
	}

	if !outcome.AuthFailed() {
		slog.Error("Failed to get SLZB-OS data", "device", deviceName, "action", action, "error", outcome.Err)
	}
}

// slzbOSErrorType maps a failed outcome to the error_type label values
// already used for the legacy endpoints, plus "auth_error" for a 401.
func slzbOSErrorType(outcome slzbos.Outcome) string {
	switch {
	case outcome.HTTPStatus == 0:
		return "request_error"
	case outcome.AuthFailed():
		return "auth_error"
	case errors.Is(outcome.Err, slzbos.ErrMalformedObject):
		return "json_error"
	case outcome.HTTPStatus == http.StatusOK:
		return "read_error"
	default:
		return "http_error"
	}
}
