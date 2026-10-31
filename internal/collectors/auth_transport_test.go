package collectors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/d0ugal/slzb-exporter/internal/config"
)

// testPassword is distinctive so that any leak is easy to spot.
const testPassword = "s3cr3t-Pa55word"

// authCapture records the Basic auth seen by a fake device.
type authCapture struct {
	user, pass string
	ok         bool
}

// newAuthEchoServer starts a server recording the Basic auth of the last
// request into got.
func newAuthEchoServer(t *testing.T, got *authCapture) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.user, got.pass, got.ok = r.BasicAuth()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return srv
}

// doGet issues a GET through a client using transport and returns the error.
func doGet(t *testing.T, transport http.RoundTripper, url string) error {
	t.Helper()

	resp, err := (&http.Client{Transport: transport}).Get(url)
	if err != nil {
		return err
	}

	_ = resp.Body.Close()

	return nil
}

// TestDeviceTransportAddsBasicAuthWithCredentials checks that configured
// credentials reach the device as an Authorization header.
func TestDeviceTransportAddsBasicAuthWithCredentials(t *testing.T) {
	var got authCapture

	srv := newAuthEchoServer(t, &got)
	transport := newDeviceTransport(config.SLZBConfig{APIURL: srv.URL, Username: "admin", Password: testPassword})

	if err := doGet(t, transport, srv.URL+"/ha_info"); err != nil {
		t.Fatalf("GET failed: %v", err)
	}

	if !got.ok || got.user != "admin" || got.pass != testPassword {
		t.Errorf("basic auth = (%q, ok=%v), want admin with the configured password", got.user, got.ok)
	}
}

// TestDeviceTransportWithoutCredentialsSendsNoAuth checks that no header is
// sent without credentials, nor with only half of the pair.
func TestDeviceTransportWithoutCredentialsSendsNoAuth(t *testing.T) {
	cases := map[string]config.SLZBConfig{
		"no credentials": {},
		"username only":  {Username: "admin"},
		"password only":  {Password: testPassword},
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			var got authCapture

			srv := newAuthEchoServer(t, &got)
			cfg.APIURL = srv.URL

			if err := doGet(t, newDeviceTransport(cfg), srv.URL+"/ha_info"); err != nil {
				t.Fatalf("GET failed: %v", err)
			}

			if got.ok {
				t.Errorf("unexpected basic auth for user %q", got.user)
			}
		})
	}
}

// TestDeviceTransportSendsNoAuthToOtherHosts checks that credentials are not
// forwarded when the device redirects to another host.
func TestDeviceTransportSendsNoAuthToOtherHosts(t *testing.T) {
	var other authCapture

	otherSrv := newAuthEchoServer(t, &other)
	device := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, otherSrv.URL+"/elsewhere", http.StatusFound)
	}))
	t.Cleanup(device.Close)

	transport := newDeviceTransport(config.SLZBConfig{APIURL: device.URL, Username: "admin", Password: testPassword})

	if err := doGet(t, transport, device.URL+"/ha_info"); err != nil {
		t.Fatalf("GET failed: %v", err)
	}

	if other.ok {
		t.Error("credentials were forwarded to a different host")
	}
}

// TestDeviceTransportNeverLeaksPassword checks that neither a transport error
// nor a formatted transport contains the password.
func TestDeviceTransportNeverLeaksPassword(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()

	transport := newDeviceTransport(config.SLZBConfig{APIURL: deadURL, Username: "admin", Password: testPassword})

	err := doGet(t, transport, deadURL+"/ha_info")
	if err == nil {
		t.Fatal("expected a transport error against a closed server")
	}

	if strings.Contains(err.Error(), testPassword) {
		t.Errorf("error string leaks the password: %v", err)
	}

	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		if out := fmt.Sprintf(verb, transport); strings.Contains(out, testPassword) {
			t.Errorf("formatting the transport with %s leaks the password: %s", verb, out)
		}
	}

	client := &http.Client{Transport: transport}
	if out := fmt.Sprintf("%+v", client); strings.Contains(out, testPassword) {
		t.Errorf("formatting the client leaks the password: %s", out)
	}
}
