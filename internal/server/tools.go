package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sv-tools/pkgsite-mcp/internal/pkgsite"
)

// Each Input struct below is the typed argument for one MCP tool. Fields
// without ",omitempty" are required; the SDK infers the JSON schema (including
// the descriptions in the `jsonschema` tags) from these types.

// SearchInput is the input for the search tool.
type SearchInput struct {
	Query  string `json:"query" jsonschema:"search query: a package name or keywords"`
	Symbol string `json:"symbol,omitempty" jsonschema:"restrict results to packages that export this symbol"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of results to return"`
	Token  string `json:"token,omitempty" jsonschema:"page token from a previous response's nextPageToken"`
}

func (s *Server) search(ctx context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, pkgsite.PaginatedResponse[pkgsite.SearchResult], error) {
	res, err := s.client.Search(ctx, in.Query, pkgsite.SearchOptions{
		Symbol:            in.Symbol,
		PaginationOptions: pkgsite.PaginationOptions{Limit: in.Limit, Token: in.Token},
	})
	return result(res, err)
}

// GetPackageInput is the input for the get_package tool.
type GetPackageInput struct {
	Path     string `json:"path" jsonschema:"import path of the package, e.g. github.com/google/uuid"`
	Version  string `json:"version,omitempty" jsonschema:"module version; defaults to the latest version"`
	Module   string `json:"module,omitempty" jsonschema:"module path to disambiguate an ambiguous package path"`
	Doc      string `json:"doc,omitempty" jsonschema:"render documentation in this format: text, md, or html"`
	Examples bool   `json:"examples,omitempty" jsonschema:"include examples (requires doc)"`
	Imports  bool   `json:"imports,omitempty" jsonschema:"include the list of imported packages"`
	Licenses bool   `json:"licenses,omitempty" jsonschema:"include license information"`
	GOOS     string `json:"goos,omitempty" jsonschema:"target GOOS for build-constrained documentation"`
	GOARCH   string `json:"goarch,omitempty" jsonschema:"target GOARCH for build-constrained documentation"`
}

func (s *Server) getPackage(ctx context.Context, _ *mcp.CallToolRequest, in GetPackageInput) (*mcp.CallToolResult, pkgsite.Package, error) {
	res, err := s.client.GetPackage(ctx, in.Path, in.Version, pkgsite.PackageOptions{
		Module:   in.Module,
		Doc:      in.Doc,
		Examples: in.Examples,
		Imports:  in.Imports,
		Licenses: in.Licenses,
		GOOS:     in.GOOS,
		GOARCH:   in.GOARCH,
	})
	return result(res, err)
}

// GetSymbolsInput is the input for the get_package_symbols tool.
type GetSymbolsInput struct {
	Path    string `json:"path" jsonschema:"import path of the package, e.g. github.com/google/uuid"`
	Version string `json:"version,omitempty" jsonschema:"module version; defaults to the latest version"`
	Module  string `json:"module,omitempty" jsonschema:"module path to disambiguate an ambiguous package path"`
	GOOS    string `json:"goos,omitempty" jsonschema:"target GOOS"`
	GOARCH  string `json:"goarch,omitempty" jsonschema:"target GOARCH"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of symbols to return"`
	Token   string `json:"token,omitempty" jsonschema:"page token from a previous response's nextPageToken"`
}

func (s *Server) getSymbols(ctx context.Context, _ *mcp.CallToolRequest, in GetSymbolsInput) (*mcp.CallToolResult, pkgsite.PaginatedResponse[pkgsite.Symbol], error) {
	res, err := s.client.GetSymbols(ctx, in.Path, in.Version, pkgsite.SymbolsOptions{
		Module:            in.Module,
		GOOS:              in.GOOS,
		GOARCH:            in.GOARCH,
		PaginationOptions: pkgsite.PaginationOptions{Limit: in.Limit, Token: in.Token},
	})
	return result(res, err)
}

// GetImportedByInput is the input for the get_imported_by tool.
type GetImportedByInput struct {
	Path    string `json:"path" jsonschema:"import path of the package to find importers of"`
	Version string `json:"version,omitempty" jsonschema:"module version; defaults to the latest version"`
	Module  string `json:"module,omitempty" jsonschema:"module path to disambiguate an ambiguous package path"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of importers to return"`
	Token   string `json:"token,omitempty" jsonschema:"page token from a previous response's nextPageToken"`
}

func (s *Server) getImportedBy(ctx context.Context, _ *mcp.CallToolRequest, in GetImportedByInput) (*mcp.CallToolResult, pkgsite.PackageImportedBy, error) {
	res, err := s.client.GetImportedBy(ctx, in.Path, in.Version, pkgsite.ImportedByOptions{
		Module:            in.Module,
		PaginationOptions: pkgsite.PaginationOptions{Limit: in.Limit, Token: in.Token},
	})
	return result(res, err)
}

// GetModuleInput is the input for the get_module tool.
type GetModuleInput struct {
	Path     string `json:"path" jsonschema:"module path, e.g. github.com/google/uuid"`
	Version  string `json:"version,omitempty" jsonschema:"module version; defaults to the latest version"`
	Readme   bool   `json:"readme,omitempty" jsonschema:"include the README contents"`
	Licenses bool   `json:"licenses,omitempty" jsonschema:"include license information"`
}

func (s *Server) getModule(ctx context.Context, _ *mcp.CallToolRequest, in GetModuleInput) (*mcp.CallToolResult, pkgsite.Module, error) {
	res, err := s.client.GetModule(ctx, in.Path, in.Version, pkgsite.ModuleOptions{
		Readme:   in.Readme,
		Licenses: in.Licenses,
	})
	return result(res, err)
}

// ListVersionsInput is the input for the list_module_versions tool.
type ListVersionsInput struct {
	Path  string `json:"path" jsonschema:"module path, e.g. github.com/google/uuid"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of versions to return"`
	Token string `json:"token,omitempty" jsonschema:"page token from a previous response's nextPageToken"`
}

func (s *Server) listVersions(ctx context.Context, _ *mcp.CallToolRequest, in ListVersionsInput) (*mcp.CallToolResult, pkgsite.PaginatedResponse[pkgsite.VersionResponse], error) {
	res, err := s.client.GetVersions(ctx, in.Path, pkgsite.PaginationOptions{Limit: in.Limit, Token: in.Token})
	return result(res, err)
}

// ListPackagesInput is the input for the list_module_packages tool.
type ListPackagesInput struct {
	ModulePath string `json:"modulePath" jsonschema:"module path, e.g. github.com/google/uuid"`
	Version    string `json:"version,omitempty" jsonschema:"module version; defaults to the latest version"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum number of packages to return"`
	Token      string `json:"token,omitempty" jsonschema:"page token from a previous response's nextPageToken"`
}

func (s *Server) listPackages(ctx context.Context, _ *mcp.CallToolRequest, in ListPackagesInput) (*mcp.CallToolResult, pkgsite.PaginatedResponse[pkgsite.ModulePackageResponse], error) {
	res, err := s.client.GetPackages(ctx, in.ModulePath, in.Version, pkgsite.PaginationOptions{Limit: in.Limit, Token: in.Token})
	return result(res, err)
}

// GetVulnsInput is the input for the get_vulnerabilities tool.
type GetVulnsInput struct {
	Path    string `json:"path" jsonschema:"module path, e.g. github.com/google/uuid"`
	Version string `json:"version,omitempty" jsonschema:"module version; defaults to the latest version"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of vulnerabilities to return"`
	Token   string `json:"token,omitempty" jsonschema:"page token from a previous response's nextPageToken"`
}

func (s *Server) getVulns(ctx context.Context, _ *mcp.CallToolRequest, in GetVulnsInput) (*mcp.CallToolResult, pkgsite.PaginatedResponse[pkgsite.Vulnerability], error) {
	res, err := s.client.GetVulns(ctx, in.Path, in.Version, pkgsite.PaginationOptions{Limit: in.Limit, Token: in.Token})
	return result(res, err)
}

// result adapts a (*T, error) client return into the (result, output, error)
// shape the SDK expects. On error it returns the zero output and the error,
// which the SDK reports in-band as a tool error the model can see.
func result[T any](v *T, err error) (*mcp.CallToolResult, T, error) {
	var zero T
	if err != nil {
		return nil, zero, err
	}
	if v == nil {
		return nil, zero, nil
	}
	return nil, *v, nil
}
