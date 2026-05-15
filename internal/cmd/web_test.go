package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseWebAccountHTML(t *testing.T) {
	html := `
		<main>
			<h4>Your account</h4>
			<dl>
				<dt>Full name</dt><dd>AJ Marson</dd>
				<dt>Mobile number</dt><dd>+1 (480) 458-8450</dd>
				<dt>Email</dt><dd><span>amarson@asu.edu</span></dd>
				<dt>Network</dt><dd>theBeach</dd>
				<dd>Created on 4/13/2026</dd>
			</dl>
		</main>`

	details := ParseWebAccountHTML(html)
	if details.FullName != "AJ Marson" {
		t.Errorf("FullName = %q", details.FullName)
	}
	if details.Phone != "+1 (480) 458-8450" {
		t.Errorf("Phone = %q", details.Phone)
	}
	if details.Email != "amarson@asu.edu" {
		t.Errorf("Email = %q", details.Email)
	}
	if details.Network != "theBeach" {
		t.Errorf("Network = %q", details.Network)
	}
	if details.CreatedOn != "4/13/2026" {
		t.Errorf("CreatedOn = %q", details.CreatedOn)
	}
}

func TestWebCookieFromEnv(t *testing.T) {
	t.Setenv("EERO_WEB_COOKIE", "session=abc; session.sig=def")

	cookie, err := webCookieFromArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cookie != "session=abc; session.sig=def" {
		t.Errorf("cookie = %q", cookie)
	}
}

func TestWebCookieFromNamedEnv(t *testing.T) {
	t.Setenv("MY_EERO_COOKIE", "session=abc")

	cookie, err := webCookieFromArgs([]string{"--cookie-env", "MY_EERO_COOKIE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cookie != "session=abc" {
		t.Errorf("cookie = %q", cookie)
	}
}

func TestWebAccountPrintsAccountDetails(t *testing.T) {
	oldURL := webAccountURL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "session=abc" {
			t.Errorf("Cookie = %q", got)
		}
		fmt.Fprint(w, `<h4>Your account</h4><dt>Full name</dt><dd>AJ Marson</dd><dt>Email</dt><dd>amarson@asu.edu</dd><dt>Network</dt><dd>theBeach</dd>`)
	}))
	defer server.Close()
	webAccountURL = server.URL
	defer func() { webAccountURL = oldURL }()

	app := newTestApp(&mockClient{})
	out := captureStdout(t, func() {
		if err := app.WebAccount([]string{"--cookie", "session=abc"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"authenticated", "AJ Marson", "amarson@asu.edu", "theBeach", "cannot be used as a mobile API token"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
