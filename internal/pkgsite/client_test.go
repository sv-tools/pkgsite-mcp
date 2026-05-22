package pkgsite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// newTestClient returns a Client pointed at a test server that records the last
// request and replies with the given status and body.
func newTestClient(t *testing.T, status int, body string) (*Client, *http.Request) {
	t.Helper()
	var last http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last = *r
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, &last
}

func TestNewValidatesServer(t *testing.T) {
	for _, server := range []string{"", "pkg.go.dev", "/relative/path", "://bad"} {
		if _, err := New(server); err == nil {
			t.Errorf("New(%q): expected error, got nil", server)
		}
	}
	if _, err := New(DefaultServer); err != nil {
		t.Errorf("New(%q): unexpected error: %v", DefaultServer, err)
	}
}

func TestGetPackageRequestAndDecode(t *testing.T) {
	const body = `{
		"modulePath": "github.com/google/uuid",
		"version": "v1.6.0",
		"isLatest": true,
		"path": "github.com/google/uuid",
		"name": "uuid",
		"synopsis": "Package uuid generates and inspects UUIDs.",
		"imports": ["fmt"]
	}`
	c, last := newTestClient(t, http.StatusOK, body)

	pkg, err := c.GetPackage(context.Background(), "github.com/google/uuid", "v1.6.0", PackageOptions{
		Doc:      "md",
		Examples: true,
		Imports:  true,
		GOOS:     "linux",
	})
	if err != nil {
		t.Fatalf("GetPackage: %v", err)
	}

	if got, want := last.URL.Path, "/v1beta/package/github.com/google/uuid"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	wantQuery := url.Values{
		"version":  {"v1.6.0"},
		"doc":      {"md"},
		"examples": {"true"},
		"imports":  {"true"},
		"goos":     {"linux"},
	}
	if got := last.URL.Query(); !equalValues(got, wantQuery) {
		t.Errorf("query = %v, want %v", got, wantQuery)
	}
	if last.Header.Get("User-Agent") != defaultUserAgent {
		t.Errorf("User-Agent = %q, want %q", last.Header.Get("User-Agent"), defaultUserAgent)
	}

	if pkg.Name != "uuid" || pkg.Path != "github.com/google/uuid" {
		t.Errorf("embedded PackageInfo not decoded: %+v", pkg.PackageInfo)
	}
	if !pkg.IsLatest || pkg.Version != "v1.6.0" {
		t.Errorf("unexpected package fields: %+v", pkg)
	}
	if len(pkg.Imports) != 1 || pkg.Imports[0] != "fmt" {
		t.Errorf("imports = %v, want [fmt]", pkg.Imports)
	}
}

func TestSearchSetsQueryParam(t *testing.T) {
	const body = `{"items":[{"packagePath":"github.com/google/uuid"}],"total":1}`
	c, last := newTestClient(t, http.StatusOK, body)

	res, err := c.Search(context.Background(), "uuid", SearchOptions{
		Symbol:            "New",
		PaginationOptions: PaginationOptions{Limit: 5, Token: "abc"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got, want := last.URL.Path, "/v1beta/search"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	q := last.URL.Query()
	for k, want := range map[string]string{"q": "uuid", "symbol": "New", "limit": "5", "token": "abc"} {
		if got := q.Get(k); got != want {
			t.Errorf("query[%q] = %q, want %q", k, got, want)
		}
	}
	if res.Total != 1 || len(res.Items) != 1 {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestGetPackagesFlattens(t *testing.T) {
	const body = `{
		"modulePath": "m",
		"packages": {
			"items": [
				{"path": "m/a", "synopsis": "A", "name": "a"},
				{"path": "m/b", "synopsis": "B", "name": "b"}
			],
			"total": 2,
			"nextPageToken": "next"
		}
	}`
	c, _ := newTestClient(t, http.StatusOK, body)

	res, err := c.GetPackages(context.Background(), "m", "", PaginationOptions{})
	if err != nil {
		t.Fatalf("GetPackages: %v", err)
	}
	if res.Total != 2 || res.NextPageToken != "next" {
		t.Errorf("pagination not flattened: %+v", res)
	}
	if len(res.Items) != 2 || res.Items[0].Path != "m/a" || res.Items[1].Synopsis != "B" {
		t.Errorf("items = %+v", res.Items)
	}
}

func TestGetSymbolsUnwraps(t *testing.T) {
	const body = `{"symbols":{"items":[{"name":"New","kind":"function"}],"total":1}}`
	c, last := newTestClient(t, http.StatusOK, body)

	res, err := c.GetSymbols(context.Background(), "p", "", SymbolsOptions{})
	if err != nil {
		t.Fatalf("GetSymbols: %v", err)
	}
	if got, want := last.URL.Path, "/v1beta/symbols/p"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if len(res.Items) != 1 || res.Items[0].Name != "New" {
		t.Errorf("symbols = %+v", res.Items)
	}
}

func TestStructuredAPIError(t *testing.T) {
	const body = `{"code":404,"message":"package not found","fixes":["check the path"]}`
	c, _ := newTestClient(t, http.StatusNotFound, body)

	_, err := c.GetPackage(context.Background(), "does/not/exist", "", PackageOptions{})
	var aerr *APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if aerr.Code != 404 || aerr.Message != "package not found" {
		t.Errorf("unexpected APIError: %+v", aerr)
	}
	if msg := aerr.Error(); msg == "" {
		t.Error("APIError.Error() returned empty string")
	}
}

func TestNonJSONErrorFallsBack(t *testing.T) {
	c, _ := newTestClient(t, http.StatusBadGateway, "upstream is down")

	_, err := c.GetModule(context.Background(), "m", "", ModuleOptions{})
	var aerr *APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if aerr.Code != http.StatusBadGateway {
		t.Errorf("Code = %d, want %d", aerr.Code, http.StatusBadGateway)
	}
}

func equalValues(a, b url.Values) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
	}
	return true
}
