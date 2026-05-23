// Package server wires the pkgsite client to an MCP server, exposing each
// pkg.go.dev API endpoint as a tool.
package server

import (
	_ "embed"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sv-tools/pkgsite-mcp/internal/pkgsite"
)

// instructions is advertised to clients during initialization. It orients the
// model on the package-vs-module distinction, error recovery, and pagination.
// The prose lives in a standalone Markdown file so it can be edited without
// touching Go code.
//
//go:embed docs/instructions.md
var instructions string

// Server holds the dependencies shared by all tool handlers.
type Server struct {
	client *pkgsite.Client
}

// New returns an MCP server that exposes the pkg.go.dev API as tools, backed by
// the given client.
func New(client *pkgsite.Client, name, version string) *mcp.Server {
	s := &Server{client: client}
	mcpServer := mcp.NewServer(
		&mcp.Implementation{Name: name, Version: version},
		&mcp.ServerOptions{Instructions: instructions},
	)
	s.registerTools(mcpServer)
	s.registerPrompts(mcpServer)
	return mcpServer
}

// ptr returns a pointer to v, for the SDK's pointer-valued hint fields.
func ptr[T any](v T) *T { return &v }

// readOnly returns the annotations shared by every tool: each one only reads
// from the live pkg.go.dev API, so hosts may auto-approve and parallelize calls.
// A fresh value is returned per tool to avoid sharing a mutable pointer.
func readOnly() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(true)}
}

func (s *Server) registerTools(m *mcp.Server) {
	mcp.AddTool(m, &mcp.Tool{
		Name:        "search",
		Description: "Search pkg.go.dev for Go packages by name or keywords. Returns matching package paths with synopses.",
		Annotations: readOnly(),
	}, s.search)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_package",
		Description: "Get details about a Go package at an import path: module path, version, synopsis, and optionally rendered documentation, imports, and licenses.",
		Annotations: readOnly(),
	}, s.getPackage)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_package_symbols",
		Description: "List the exported symbols (functions, types, methods, variables, constants) of a Go package.",
		Annotations: readOnly(),
	}, s.getSymbols)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_imported_by",
		Description: "List packages from other modules that import the given package (its reverse dependencies). Importers within the same module are excluded by the API.",
		Annotations: readOnly(),
	}, s.getImportedBy)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_module",
		Description: "Get details about a Go module: version, commit time, repository URL, and optionally README and license information.",
		Annotations: readOnly(),
	}, s.getModule)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "list_module_versions",
		Description: "List the available versions of a Go module.",
		Annotations: readOnly(),
	}, s.listVersions)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "list_module_packages",
		Description: "List the packages contained in a Go module at a given version.",
		Annotations: readOnly(),
	}, s.listPackages)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_vulnerabilities",
		Description: "List known vulnerabilities (from the Go vulnerability database) affecting a module path and version.",
		Annotations: readOnly(),
	}, s.getVulns)
}
