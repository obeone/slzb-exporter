package collectors

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d0ugal/promexporter/app"
	promexporter_metrics "github.com/d0ugal/promexporter/metrics"
	"github.com/d0ugal/slzb-exporter/internal/config"
	"github.com/d0ugal/slzb-exporter/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

// This file holds the integration test harness: a fake SLZB device serving
// canned answers per route, a collector wired to it exactly as in production
// (minus app.Build, so tracing stays off) and helpers to query the metrics.
// Tests run one collection cycle at a time on the test goroutine, so the only
// concurrency is the fake server's handlers, which lock their own state.

// Routes of the fake device, as the collector requests them.
const (
	routeLegacy    = "/api?action=0&page=0"
	routeConfig    = "/api?action=4&page=0"
	routeHAInfo    = "/ha_info"
	routeHASensors = "/ha_sensors"
)

// osFixtureDir holds the /ha_* fixtures shared with the slzbos unit tests;
// reusing them keeps a single copy of each payload.
const osFixtureDir = "../slzbos/testdata"

// fakeResponse is the canned answer of the fake device for one route.
type fakeResponse struct {
	status int
	body   string
	// headers are set on the response, e.g. respValuesArr.
	headers map[string]string
	// dropConnection closes the connection without answering, which the
	// client sees as a transport error.
	dropConnection bool
}

// recordedRequest is what the fake device remembers about one request.
type recordedRequest struct {
	route         string
	authorization string
}

// fakeDevice is an httptest server standing in for an SLZB device. Routes can
// be changed between cycles; every request is recorded with its route and
// Authorization header.
type fakeDevice struct {
	server *httptest.Server

	mu       sync.Mutex
	routes   map[string]fakeResponse
	recorded []recordedRequest
	// username and password, when set, make the device answer 401 to any
	// request without the matching Basic credentials, like SLZB-OS v3 with a
	// web password.
	username, password string
}

// newFakeDevice starts a fake device serving routes; it is closed when the
// test ends.
func newFakeDevice(t *testing.T, routes map[string]fakeResponse) *fakeDevice {
	t.Helper()

	d := &fakeDevice{routes: routes}
	d.server = httptest.NewServer(d)
	t.Cleanup(d.server.Close)

	return d
}

// URL returns the base URL to configure as the device API URL.
func (d *fakeDevice) URL() string {
	return d.server.URL
}

// setRoute replaces the answer for route, typically between two cycles.
func (d *fakeDevice) setRoute(route string, resp fakeResponse) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.routes[route] = resp
}

// requireAuth makes every route answer 401 unless the request carries these
// Basic credentials.
func (d *fakeDevice) requireAuth(username, password string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.username, d.password = username, password
}

// requests returns a copy of every request received so far.
func (d *fakeDevice) requests() []recordedRequest {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]recordedRequest(nil), d.recorded...)
}

// requestCount returns how many requests reached route.
func (d *fakeDevice) requestCount(route string) int {
	n := 0

	for _, req := range d.requests() {
		if req.route == route {
			n++
		}
	}

	return n
}

// ServeHTTP records the request, then answers with the configured response,
// a 401 when credentials are required and missing, or a 404 for an unknown
// route.
func (d *fakeDevice) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := routeOf(r)

	d.mu.Lock()
	d.recorded = append(d.recorded, recordedRequest{route: route, authorization: r.Header.Get("Authorization")})
	resp, known := d.routes[route]
	authorized := d.authorized(r)
	d.mu.Unlock()

	switch {
	case !authorized:
		resp = fakeResponse{status: http.StatusUnauthorized, body: "wrong login or password"}
	case !known:
		resp = fakeResponse{status: http.StatusNotFound, body: "Not found"}
	}

	writeFakeResponse(w, resp)
}

// authorized reports whether r satisfies the auth requirement. The caller
// holds d.mu.
func (d *fakeDevice) authorized(r *http.Request) bool {
	if d.username == "" {
		return true
	}

	user, pass, ok := r.BasicAuth()

	return ok && user == d.username && pass == d.password
}

// routeOf returns the path plus raw query, the key of the routes map.
func routeOf(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return r.URL.Path
	}

	return r.URL.Path + "?" + r.URL.RawQuery
}

// writeFakeResponse sends resp, or drops the connection when asked to.
func writeFakeResponse(w http.ResponseWriter, resp fakeResponse) {
	if resp.dropConnection {
		dropConnection(w)
		return
	}

	for name, value := range resp.headers {
		w.Header().Set(name, value)
	}

	w.WriteHeader(resp.status)
	_, _ = w.Write([]byte(resp.body))
}

// dropConnection closes the underlying connection without writing a status
// line, so the client gets a transport error rather than an HTTP answer.
func dropConnection(w http.ResponseWriter) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		panic("fake device: response writer cannot be hijacked")
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		panic("fake device: hijack failed: " + err.Error())
	}

	_ = conn.Close()
}

// readFixture returns a file of this package's testdata directory.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	return readFile(t, filepath.Join("testdata", name))
}

// readOSFixture returns a /ha_* fixture shared with the slzbos package.
func readOSFixture(t *testing.T, name string) []byte {
	t.Helper()

	return readFile(t, filepath.Join(osFixtureDir, name))
}

// readFile reads path or fails the test.
func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	return data
}

// okResponse answers 200 with body.
func okResponse(body []byte) fakeResponse {
	return fakeResponse{status: http.StatusOK, body: string(body)}
}

// legacyValuesResponse answers the legacy dashboard request the way SLZB-06
// firmware does: a 200 whose data travels in the respValuesArr header.
func legacyValuesResponse(header string) fakeResponse {
	return fakeResponse{
		status:  http.StatusOK,
		body:    "<html></html>",
		headers: map[string]string{"respValuesArr": header},
	}
}

// legacyHeader builds a respValuesArr header value from the SLZB-06 fixture,
// with overrides applied. The value is single-line JSON, as a header must be.
func legacyHeader(t *testing.T, overrides map[string]string) string {
	t.Helper()

	values := map[string]string{}
	if err := json.Unmarshal(readFixture(t, "legacy_resparr_slzb06.json"), &values); err != nil {
		t.Fatalf("decode legacy fixture: %v", err)
	}

	for key, value := range overrides {
		values[key] = value
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("encode legacy header: %v", err)
	}

	return string(encoded)
}

// editPayload decodes a /ha_* payload, lets edit change the object stored
// under its top-level key ("Info" or "Sensors") and re-encodes it. Numbers
// are kept as json.Number so their text survives the round trip.
func editPayload(t *testing.T, payload []byte, object string, edit func(map[string]any)) []byte {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()

	var doc map[string]map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	edit(doc[object])

	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	return encoded
}

// ultima3Routes describes an SLZB-Ultima3 on SLZB-OS v3: no legacy endpoint,
// full /ha_info and /ha_sensors.
func ultima3Routes(t *testing.T) map[string]fakeResponse {
	t.Helper()

	return map[string]fakeResponse{
		routeLegacy:    {status: http.StatusNotFound, body: "Not found"},
		routeHAInfo:    okResponse(readOSFixture(t, "ha_info_ultima3.json")),
		routeHASensors: okResponse(readOSFixture(t, "ha_sensors_ultima3.json")),
	}
}

// legacyOnlyRoutes describes an SLZB-06 on core firmware older than 2.x:
// legacy dashboard and file list, and "URL NOT FOUND" (with a 200, as
// SMLIGHT firmware may send it) for /ha_info.
func legacyOnlyRoutes(t *testing.T) map[string]fakeResponse {
	t.Helper()

	return map[string]fakeResponse{
		routeLegacy: legacyValuesResponse(legacyHeader(t, nil)),
		routeConfig: okResponse(readFixture(t, "config_files.json")),
		routeHAInfo: {status: http.StatusOK, body: "URL NOT FOUND"},
	}
}

// slzb06CoreRoutes describes an SLZB-06 on core firmware 2.x: both the legacy
// endpoints and the single-radio /ha_* payloads answer.
func slzb06CoreRoutes(t *testing.T) map[string]fakeResponse {
	t.Helper()

	routes := legacyOnlyRoutes(t)
	routes[routeHAInfo] = okResponse(readOSFixture(t, "ha_info_no_radios.json"))
	routes[routeHASensors] = okResponse(readOSFixture(t, "ha_sensors_slzb06.json"))

	return routes
}

// collectorOptions are the per-test knobs of newTestCollector.
type collectorOptions struct {
	username string
	password string
}

// testCollector is a production SLZBCollector plus the registry it writes to.
type testCollector struct {
	*SLZBCollector

	registry *metrics.SLZBRegistry
}

// newTestCollector builds a collector against serverURL on a fresh registry.
// app.New is used without Build(), so GetTracer returns nil and every span
// branch is skipped, and the end-of-cycle delay is removed.
func newTestCollector(t *testing.T, serverURL string, opts collectorOptions) *testCollector {
	t.Helper()

	cfg := &config.Config{
		SLZB: config.SLZBConfig{
			APIURL:   serverURL,
			Interval: time.Second,
			Username: opts.username,
			Password: opts.password,
		},
	}

	registry := metrics.NewSLZBRegistry(promexporter_metrics.NewRegistry("slzb_exporter_info"))
	collector := NewSLZBCollector(cfg, registry, app.New("test"))
	collector.interRequestDelay = 0

	return &testCollector{SLZBCollector: collector, registry: registry}
}

// collect runs one collection cycle synchronously.
func (tc *testCollector) collect() {
	tc.collectMetrics(context.Background())
}

// labels returns {"device": <collector device id>} plus the given pairs.
func (tc *testCollector) labels(pairs ...string) prometheus.Labels {
	return deviceLabels(tc.deviceID, pairs...)
}

// gatheredNames returns the sorted names of the families currently exported
// by the collector's registry whose name starts with prefix. Prometheus omits
// vecs without any series, so this is the set a scrape would show.
func (tc *testCollector) gatheredNames(t *testing.T, prefix string) []string {
	t.Helper()

	families, err := tc.registry.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var names []string

	for _, family := range families {
		if strings.HasPrefix(family.GetName(), prefix) {
			names = append(names, family.GetName())
		}
	}

	sort.Strings(names)

	return names
}

// matchingSeries returns the series of c whose labels include every pair of
// want. It gathers c through its own registry, so looking a series up never
// creates it (unlike vec.With).
func matchingSeries(t *testing.T, c prometheus.Collector, want prometheus.Labels) []*dto.Metric {
	t.Helper()

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var matches []*dto.Metric

	for _, family := range families {
		for _, metric := range family.GetMetric() {
			if hasLabels(metric, want) {
				matches = append(matches, metric)
			}
		}
	}

	return matches
}

// hasLabels reports whether metric carries every name/value pair of want.
func hasLabels(metric *dto.Metric, want prometheus.Labels) bool {
	got := map[string]string{}
	for _, pair := range metric.GetLabel() {
		got[pair.GetName()] = pair.GetValue()
	}

	for name, value := range want {
		if got[name] != value {
			return false
		}
	}

	return true
}

// assertValue checks that exactly one series of c matches labels and that
// its gauge or counter value is want.
func assertValue(t *testing.T, c prometheus.Collector, labels prometheus.Labels, want float64) {
	t.Helper()

	matches := matchingSeries(t, c, labels)
	if len(matches) != 1 {
		t.Errorf("series matching %v: got %d, want exactly 1", labels, len(matches))
		return
	}

	if got := seriesValue(matches[0]); got != want {
		t.Errorf("value of %v = %v, want %v", labels, got, want)
	}
}

// seriesValue returns the value of a gauge or counter sample.
func seriesValue(metric *dto.Metric) float64 {
	if metric.GetCounter() != nil {
		return metric.GetCounter().GetValue()
	}

	return metric.GetGauge().GetValue()
}

// assertNoSeries checks that no series of c matches labels.
func assertNoSeries(t *testing.T, c prometheus.Collector, labels prometheus.Labels) {
	t.Helper()

	if matches := matchingSeries(t, c, labels); len(matches) != 0 {
		t.Errorf("series matching %v: got %d, want none", labels, len(matches))
	}
}

// assertSeriesCount checks the total number of series exported by c.
func assertSeriesCount(t *testing.T, c prometheus.Collector, want int) {
	t.Helper()

	if got := testutil.CollectAndCount(c); got != want {
		t.Errorf("series count = %d, want %d", got, want)
	}
}

// logBuffer is a goroutine-safe sink for captured slog output.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write implements io.Writer under the buffer lock.
func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// String returns everything logged so far.
func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// countLines returns how many logged lines contain every one of parts.
func (b *logBuffer) countLines(parts ...string) int {
	n := 0

	for _, line := range strings.Split(b.String(), "\n") {
		if containsAll(line, parts) {
			n++
		}
	}

	return n
}

// containsAll reports whether s contains every one of parts.
func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}

	return true
}

// captureLogs routes the default slog logger, at debug level, to a buffer
// for the rest of the test. The collector logs through the package-level
// slog functions, so swapping the default logger is the only hook; tests
// using it must not run in parallel.
func captureLogs(t *testing.T) *logBuffer {
	t.Helper()

	buf := &logBuffer{}
	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return buf
}
