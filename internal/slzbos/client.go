package slzbos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Endpoint paths, relative to the device base URL.
const (
	InfoPath    = "/ha_info"
	SensorsPath = "/ha_sensors"
)

// maxBodyBytes caps how much of a response is read. Real /ha_* payloads are
// a few kilobytes; the cap protects the exporter from a misbehaving device or
// proxy streaming an unbounded body.
const maxBodyBytes = 1 << 20

// urlNotFoundBody is what SMLIGHT firmware answers for an unknown route,
// sometimes with a non-404 status (pysmlight checks the body, not the code).
const urlNotFoundBody = "URL NOT FOUND"

// Status classifies the result of one endpoint fetch.
type Status int

const (
	// StatusOK means the endpoint answered and its payload was decoded.
	StatusOK Status = iota
	// StatusUnsupported means the firmware does not serve the endpoint. It is
	// an expected condition on legacy firmware, not an error to count.
	StatusUnsupported
	// StatusFailed means the endpoint exists or may exist but the fetch went
	// wrong: transport error, unexpected HTTP status, 401, malformed payload.
	StatusFailed
)

// String returns a lowercase name suitable for logs and span attributes.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusUnsupported:
		return "unsupported"
	case StatusFailed:
		return "failed"
	default:
		return fmt.Sprintf("status(%d)", int(s))
	}
}

// Outcome describes how a fetch went, independently of the decoded data.
type Outcome struct {
	Status Status
	// HTTPStatus is the response status code, 0 when no response arrived.
	HTTPStatus int
	// Err explains a non-OK status; it is also set for StatusUnsupported so
	// callers can log the reason at debug level. It never contains the
	// response body nor any credential.
	Err error
}

// AuthFailed reports whether the device rejected the credentials (HTTP 401),
// so callers can surface a configuration hint instead of a generic error.
func (o Outcome) AuthFailed() bool {
	return o.HTTPStatus == http.StatusUnauthorized
}

// Client fetches the SLZB-OS Home Assistant endpoints of one device.
//
// It does not handle authentication: the caller supplies an *http.Client
// whose transport adds credentials (and tracing) as needed, which keeps
// secrets out of this package entirely.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient returns a Client for the device at baseURL (e.g.
// "http://192.0.2.1"), issuing requests through httpClient.
func NewClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// FetchInfo GETs /ha_info and returns the normalised Info when the outcome
// status is StatusOK, nil otherwise.
func (c *Client) FetchInfo(ctx context.Context) (*Info, Outcome) {
	return fetch(ctx, c, InfoPath, ParseInfo)
}

// FetchSensors GETs /ha_sensors and returns the decoded Sensors when the
// outcome status is StatusOK, nil otherwise.
func (c *Client) FetchSensors(ctx context.Context) (*Sensors, Outcome) {
	return fetch(ctx, c, SensorsPath, ParseSensors)
}

// fetch performs the GET for path, classifies the response and, on a 200,
// decodes the body with parse.
func fetch[T any](ctx context.Context, c *Client, path string, parse func([]byte) (*T, error)) (*T, Outcome) {
	statusCode, body, err := c.get(ctx, path)
	if err != nil {
		return nil, Outcome{Status: StatusFailed, HTTPStatus: statusCode, Err: err}
	}

	if outcome, done := classifyResponse(path, statusCode, body); done {
		return nil, outcome
	}

	data, err := parse(body)
	if err != nil {
		return nil, parseOutcome(statusCode, err)
	}

	return data, Outcome{Status: StatusOK, HTTPStatus: statusCode}
}

// get issues the request and reads the (capped) body. The returned status
// code is 0 when no response was received.
func (c *Client) get(ctx context.Context, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("slzbos: build request for %s: %w", path, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("slzbos: GET %s: %w", path, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("slzbos: read %s body: %w", path, err)
	}

	return resp.StatusCode, body, nil
}

// classifyResponse decides the outcome from the status code and body before
// any JSON decoding. It returns done=false only for a 200 that must be parsed.
//
// The "URL NOT FOUND" body check comes first because SMLIGHT firmware may send
// it with a 200 status, which would otherwise be read as a malformed payload.
func classifyResponse(path string, statusCode int, body []byte) (Outcome, bool) {
	switch {
	case strings.TrimSpace(string(body)) == urlNotFoundBody:
		return unsupported(statusCode, fmt.Errorf("slzbos: %s answered %q", path, urlNotFoundBody)), true
	case statusCode == http.StatusNotFound:
		return unsupported(statusCode, fmt.Errorf("slzbos: %s answered HTTP 404", path)), true
	case statusCode != http.StatusOK:
		return Outcome{
			Status:     StatusFailed,
			HTTPStatus: statusCode,
			Err:        fmt.Errorf("slzbos: %s answered HTTP %d", path, statusCode),
		}, true
	default:
		return Outcome{}, false
	}
}

// parseOutcome maps a decoding error to an outcome: a missing object means
// the endpoint is not really there (e.g. an HTML page), anything else is a
// broken payload.
func parseOutcome(statusCode int, err error) Outcome {
	if errors.Is(err, ErrMissingObject) {
		return unsupported(statusCode, err)
	}

	return Outcome{Status: StatusFailed, HTTPStatus: statusCode, Err: err}
}

// unsupported builds a StatusUnsupported outcome carrying its reason.
func unsupported(statusCode int, reason error) Outcome {
	return Outcome{Status: StatusUnsupported, HTTPStatus: statusCode, Err: reason}
}
