package collectors

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/d0ugal/slzb-exporter/internal/config"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// authHint tells the operator how to fix a 401 without ever echoing a secret.
// Configuration is read from environment variables only, so the hint names
// nothing else.
const authHint = "set SLZB_EXPORTER_SLZB_USERNAME and SLZB_EXPORTER_SLZB_PASSWORD " +
	"to the device web UI credentials"

// basicAuthTransport adds HTTP Basic auth to every request sent to the device.
//
// Credentials are attached per request rather than embedded in the API URL so
// they never appear in URLs, which end up in error strings, logs and span
// attributes. They are only added for the configured device host: the
// transport also sees redirected requests, and net/http's own stripping of
// Authorization on cross-host redirects would otherwise be undone here.
type basicAuthTransport struct {
	base     http.RoundTripper
	host     string
	username string
	password string
}

// newDeviceTransport returns the transport used for every device request:
// the OpenTelemetry-instrumented default transport, wrapped with Basic auth
// only when both credentials are configured.
func newDeviceTransport(cfg config.SLZBConfig) http.RoundTripper {
	base := otelhttp.NewTransport(http.DefaultTransport)
	if !cfg.HasAuth() {
		return base
	}

	return &basicAuthTransport{
		base:     base,
		host:     deviceHost(cfg.APIURL),
		username: cfg.Username,
		password: cfg.Password,
	}
}

// deviceHost extracts host[:port] from the API URL, "" when it cannot be
// parsed (auth is then never sent, which fails closed).
func deviceHost(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil {
		return ""
	}

	return u.Host
}

// RoundTrip implements http.RoundTripper. The request is cloned because a
// RoundTripper must not modify the caller's request.
func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.host == "" || req.URL.Host != t.host {
		return t.base.RoundTrip(req)
	}

	authed := req.Clone(req.Context())
	authed.SetBasicAuth(t.username, t.password)

	return t.base.RoundTrip(authed)
}

// CloseIdleConnections forwards to the wrapped transport so that
// http.Client.CloseIdleConnections keeps working through the wrapper.
func (t *basicAuthTransport) CloseIdleConnections() {
	type idleCloser interface{ CloseIdleConnections() }
	if c, ok := t.base.(idleCloser); ok {
		c.CloseIdleConnections()
	}
}

// String redacts the password so that printing the transport (or a struct
// holding it) with %v or %+v cannot leak it.
func (t *basicAuthTransport) String() string {
	return "basicAuthTransport{host:" + t.host + " username:" + t.username + " password:[REDACTED]}"
}

// GoString redacts the password for %#v as well.
func (t *basicAuthTransport) GoString() string {
	return t.String()
}

// logAuthFailure emits the single per-cycle warning for a device answering
// 401, whichever endpoints rejected the request. Only whether credentials are
// configured is logged, never their values.
func (sc *SLZBCollector) logAuthFailure(deviceName string) {
	slog.Warn("SLZB device rejected the request with HTTP 401",
		"device", deviceName,
		"credentials_configured", sc.config.SLZB.HasAuth(),
		"hint", authHint)
}
