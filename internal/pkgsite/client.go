package pkgsite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DefaultServer is the public pkg.go.dev API server.
const DefaultServer = "https://pkg.go.dev"

const defaultUserAgent = "pkgsite-mcp"

// maxErrorBody bounds how much of an error response body we read.
const maxErrorBody = 1 << 20 // 1 MiB

// Client fetches data from the pkg.go.dev v1beta API.
type Client struct {
	server     *url.URL
	httpClient *http.Client
	userAgent  string
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
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// get fetches u and decodes the JSON response into dst. Non-200 responses are
// decoded into an *APIError when possible.
func (c *Client) get(ctx context.Context, u *url.URL, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		var aerr APIError
		if json.Unmarshal(body, &aerr) == nil && aerr.Message != "" {
			if aerr.Code == 0 {
				aerr.Code = resp.StatusCode
			}
			return &aerr
		}
		return &APIError{Code: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
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
	for _, p := range resp.Packages.Items {
		items = append(items, ModulePackageResponse{Path: p.Path, Synopsis: p.Synopsis})
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
