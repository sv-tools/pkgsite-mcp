// Package server wires the pkgsite client to an MCP server, exposing each
// pkg.go.dev API endpoint as a tool.
package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sv-tools/pkgsite-mcp/internal/pkgsite"
)

// Server holds the dependencies shared by all tool handlers.
type Server struct {
	client *pkgsite.Client
}

// New returns an MCP server that exposes the pkg.go.dev API as tools, backed by
// the given client.
func New(client *pkgsite.Client, name, version string) *mcp.Server {
	s := &Server{client: client}
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, nil)
	s.registerTools(mcpServer)
	return mcpServer
}

func (s *Server) registerTools(m *mcp.Server) {
	mcp.AddTool(m, &mcp.Tool{
		Name:        "search",
		Description: "Search pkg.go.dev for Go packages by name or keywords. Returns matching package paths with synopses.",
	}, s.search)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_package",
		Description: "Get details about a Go package at an import path: module path, version, synopsis, and optionally rendered documentation, imports, and licenses.",
	}, s.getPackage)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_package_symbols",
		Description: "List the exported symbols (functions, types, methods, variables, constants) of a Go package.",
	}, s.getSymbols)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_imported_by",
		Description: "List packages from other modules that import the given package (its reverse dependencies). Importers within the same module are excluded by the API.",
	}, s.getImportedBy)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_module",
		Description: "Get details about a Go module: version, commit time, repository URL, and optionally README and license information.",
	}, s.getModule)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "list_module_versions",
		Description: "List the available versions of a Go module.",
	}, s.listVersions)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "list_module_packages",
		Description: "List the packages contained in a Go module at a given version.",
	}, s.listPackages)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_vulnerabilities",
		Description: "List known vulnerabilities (from the Go vulnerability database) affecting a module path and version.",
	}, s.getVulns)
}
