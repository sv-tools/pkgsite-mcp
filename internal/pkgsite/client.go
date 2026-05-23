package pkgsite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DefaultServer is the public pkg.go.dev API server.
const DefaultServer = "https://pkg.go.dev"

const defaultUserAgent = "pkgsite-mcp"

const (
	// maxErrorBody bounds how much of an error response body we read.
	maxErrorBody = 1 << 20 // 1 MiB
	// maxBodyBytes bounds how much of a success response body we buffer.
	maxBodyBytes = 8 << 20 // 8 MiB
	// defaultRetryBase is the base delay for exponential backoff.
	defaultRetryBase = 200 * time.Millisecond
	// maxRetryDelay caps any single backoff wait.
	maxRetryDelay = 5 * time.Second
)

// Client fetches data from the pkg.go.dev v1beta API.
type Client struct {
	server     *url.URL
	httpClient *http.Client
	userAgent  string
	maxRetries int           // additional attempts after the first; 0 disables retries
	retryBase  time.Duration // base delay for exponential backoff
	cache      *cache        // response cache; nil disables caching
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client used for requests. The default client has
// a 30s timeout.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithUserAgent sets the User-Agent header sent with each request.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// WithRetry retries transient failures (network errors and HTTP 429/5xx) up to
// maxRetries additional times, with exponential backoff starting at baseDelay.
// A maxRetries of 0 disables retries. A baseDelay <= 0 keeps the default.
func WithRetry(maxRetries int, baseDelay time.Duration) Option {
	return func(c *Client) {
		if maxRetries >= 0 {
			c.maxRetries = maxRetries
		}
		if baseDelay > 0 {
			c.retryBase = baseDelay
		}
	}
}

// WithCache caches successful responses for ttl, keyed by request URL, bounded
// to maxEntries. Because version-pinned data is immutable and "latest" lookups
// tolerate a short staleness window, this safely avoids refetching during an
// assistant's multi-step exploration. A non-positive ttl or maxEntries disables
// caching.
func WithCache(ttl time.Duration, maxEntries int) Option {
	return func(c *Client) {
		if ttl > 0 && maxEntries > 0 {
			c.cache = newCache(ttl, maxEntries)
		}
	}
}

// New creates a Client that talks to the given server (e.g. DefaultServer).
func New(server string, opts ...Option) (*Client, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("parsing server URL %q: %w", server, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("server URL %q must be absolute", server)
	}
	c := &Client{
		server:     u,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  defaultUserAgent,
		retryBase:  defaultRetryBase,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// get fetches u and decodes the JSON response into dst, serving from cache when
// enabled and retrying transient failures. Non-200 responses are decoded into an
// *APIError when possible.
func (c *Client) get(ctx context.Context, u *url.URL, dst any) error {
	key := u.String()
	if c.cache != nil {
		if body, ok := c.cache.get(key); ok {
			return decode(body, dst)
		}
	}
	body, err := c.fetch(ctx, u)
	if err != nil {
		return err
	}
	if c.cache != nil {
		c.cache.set(key, body)
	}
	return decode(body, dst)
}

func decode(body []byte, dst any) error {
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// fetch performs the GET, retrying transient failures up to c.maxRetries times.
func (c *Client) fetch(ctx context.Context, u *url.URL) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		body, retryAfter, retryable, err := c.try(ctx, u)
		if err == nil {
			return body, nil
		}
		if !retryable || attempt >= c.maxRetries {
			return nil, err
		}
		if werr := wait(ctx, c.backoff(attempt, retryAfter)); werr != nil {
			return nil, err
		}
	}
}

// try performs a single GET attempt. On a 200 it returns the body bytes. On a
// non-200 or transport error it returns an error; retryable reports whether the
// caller should retry, and retryAfter is any server-advised delay.
func (c *Client) try(ctx context.Context, u *url.URL) (body []byte, retryAfter time.Duration, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Transport error: retry unless the context itself was cancelled.
		return nil, 0, ctx.Err() == nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		b, rerr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		if rerr != nil {
			return nil, 0, ctx.Err() == nil, fmt.Errorf("reading response: %w", rerr)
		}
		return b, 0, false, nil
	}

	errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	// Drain any remainder so the connection can be reused (keep-alive).
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil, parseRetryAfter(resp.Header.Get("Retry-After")), retryableStatus(resp.StatusCode), parseAPIError(resp.StatusCode, errBody)
}

// parseAPIError builds a structured error from a non-200 response body, falling
// back to the HTTP status text when the body is not a recognizable API error.
func parseAPIError(status int, body []byte) *APIError {
	var aerr APIError
	if json.Unmarshal(body, &aerr) == nil && aerr.Message != "" {
		if aerr.Code == 0 {
			aerr.Code = status
		}
		return &aerr
	}
	return &APIError{Code: status, Message: http.StatusText(status)}
}

// retryableStatus reports whether an HTTP status warrants a retry.
func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// parseRetryAfter interprets a Retry-After header (delay-seconds or HTTP-date),
// returning 0 when absent or unparseable.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// backoff returns the delay before retry attempt+1, preferring a server-advised
// Retry-After and otherwise using exponential backoff with full jitter.
func (c *Client) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return min(retryAfter, maxRetryDelay)
	}
	d := c.retryBase << attempt
	if d <= 0 || d > maxRetryDelay {
		d = maxRetryDelay
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

// wait sleeps for d, returning early if ctx is cancelled.
func wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// endpoint builds a request URL for the given path segments and query values.
func (c *Client) endpoint(query url.Values, segments ...string) *url.URL {
	u := c.server.JoinPath(segments...)
	u.RawQuery = query.Encode()
	return u
}

func setPagination(q url.Values, limit int, token string) {
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if token != "" {
		q.Set("token", token)
	}
}

// PaginationOptions are common options for paginated endpoints.
type PaginationOptions struct {
	Limit int    // maximum items per page; 0 means the server default
	Token string // page token from a previous response's NextPageToken
}

// PackageOptions are options for GetPackage.
type PackageOptions struct {
	Module   string // disambiguate the module path for an ambiguous package path
	Doc      string // render docs in this format: "text", "md", or "html"
	Examples bool   // include examples (requires Doc)
	Imports  bool   // list imported packages
	Licenses bool   // include license information
	GOOS     string // target GOOS
	GOARCH   string // target GOARCH
}

// GetPackage fetches information about a package at the given path and version.
// An empty version means the latest version.
func (c *Client) GetPackage(ctx context.Context, path, version string, opts PackageOptions) (*Package, error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Module != "" {
		q.Set("module", opts.Module)
	}
	if opts.Doc != "" {
		q.Set("doc", opts.Doc)
	}
	if opts.Examples {
		q.Set("examples", "true")
	}
	if opts.Imports {
		q.Set("imports", "true")
	}
	if opts.Licenses {
		q.Set("licenses", "true")
	}
	if opts.GOOS != "" {
		q.Set("goos", opts.GOOS)
	}
	if opts.GOARCH != "" {
		q.Set("goarch", opts.GOARCH)
	}
	var resp Package
	if err := c.get(ctx, c.endpoint(q, "v1beta", "package", path), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SymbolsOptions are options for GetSymbols.
type SymbolsOptions struct {
	Module string
	GOOS   string
	GOARCH string
	PaginationOptions
}

// GetSymbols fetches the exported symbols of a package.
func (c *Client) GetSymbols(ctx context.Context, path, version string, opts SymbolsOptions) (*PaginatedResponse[Symbol], error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Module != "" {
		q.Set("module", opts.Module)
	}
	if opts.GOOS != "" {
		q.Set("goos", opts.GOOS)
	}
	if opts.GOARCH != "" {
		q.Set("goarch", opts.GOARCH)
	}
	setPagination(q, opts.Limit, opts.Token)
	var resp PackageSymbols
	if err := c.get(ctx, c.endpoint(q, "v1beta", "symbols", path), &resp); err != nil {
		return nil, err
	}
	return &resp.Symbols, nil
}

// ImportedByOptions are options for GetImportedBy.
type ImportedByOptions struct {
	Module string
	PaginationOptions
}

// GetImportedBy fetches the packages that import the given package (reverse
// dependencies).
func (c *Client) GetImportedBy(ctx context.Context, path, version string, opts ImportedByOptions) (*PackageImportedBy, error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Module != "" {
		q.Set("module", opts.Module)
	}
	setPagination(q, opts.Limit, opts.Token)
	var resp PackageImportedBy
	if err := c.get(ctx, c.endpoint(q, "v1beta", "imported-by", path), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ModuleOptions are options for GetModule.
type ModuleOptions struct {
	Readme   bool // include the README contents
	Licenses bool // include license information
}

// GetModule fetches information about a module at the given path and version.
// An empty version means the latest version.
func (c *Client) GetModule(ctx context.Context, path, version string, opts ModuleOptions) (*Module, error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Readme {
		q.Set("readme", "true")
	}
	if opts.Licenses {
		q.Set("licenses", "true")
	}
	var resp Module
	if err := c.get(ctx, c.endpoint(q, "v1beta", "module", path), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetVersions fetches the list of versions for the given module path.
func (c *Client) GetVersions(ctx context.Context, path string, opts PaginationOptions) (*PaginatedResponse[VersionResponse], error) {
	q := make(url.Values)
	setPagination(q, opts.Limit, opts.Token)
	var resp PaginatedResponse[VersionResponse]
	if err := c.get(ctx, c.endpoint(q, "v1beta", "versions", path), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetVulns fetches known vulnerabilities for the given module path and version.
func (c *Client) GetVulns(ctx context.Context, path, version string, opts PaginationOptions) (*PaginatedResponse[Vulnerability], error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	setPagination(q, opts.Limit, opts.Token)
	var resp PaginatedResponse[Vulnerability]
	if err := c.get(ctx, c.endpoint(q, "v1beta", "vulns", path), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPackages fetches the packages contained in the given module path and
// version.
func (c *Client) GetPackages(ctx context.Context, modulePath, version string, opts PaginationOptions) (*PaginatedResponse[ModulePackageResponse], error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	setPagination(q, opts.Limit, opts.Token)
	var resp PackagesResponse
	if err := c.get(ctx, c.endpoint(q, "v1beta", "packages", modulePath), &resp); err != nil {
		return nil, err
	}
	items := make([]ModulePackageResponse, 0, len(resp.Packages.Items))
	for i := range resp.Packages.Items {
		p := &resp.Packages.Items[i]
		items = append(items, ModulePackageResponse{Path: p.Path, Name: p.Name, Synopsis: p.Synopsis})
	}
	return &PaginatedResponse[ModulePackageResponse]{
		Items:         items,
		Total:         resp.Packages.Total,
		NextPageToken: resp.Packages.NextPageToken,
	}, nil
}

// SearchOptions are options for Search.
type SearchOptions struct {
	Symbol string // restrict results to packages exporting this symbol
	PaginationOptions
}

// Search queries pkg.go.dev for packages matching query.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) (*PaginatedResponse[SearchResult], error) {
	q := make(url.Values)
	q.Set("q", query)
	if opts.Symbol != "" {
		q.Set("symbol", opts.Symbol)
	}
	setPagination(q, opts.Limit, opts.Token)
	var resp PaginatedResponse[SearchResult]
	if err := c.get(ctx, c.endpoint(q, "v1beta", "search"), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
