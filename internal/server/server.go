// Package server wires the pkgsite client to an MCP server, exposing each
// pkg.go.dev API endpoint as a tool.
package server

import (
	"embed"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sv-tools/pkgsite-mcp/internal/pkgsite"
)

// docsFS holds the tool and server prose. Keeping the text in standalone
// Markdown files lets it be edited without touching Go code.
//
//go:embed docs
var docsFS embed.FS

// doc returns the trimmed contents of an embedded Markdown file under docs/. It
// panics if the file is missing, since the docs are embedded at build time and
// an absent file is a packaging bug, surfaced immediately by New.
func doc(name string) string {
	b, err := docsFS.ReadFile("docs/" + name)
	if err != nil {
		panic("server: missing embedded doc " + name + ": " + err.Error())
	}
	return strings.TrimSpace(string(b))
}

// instructions is advertised to clients during initialization. It orients the
// model on the package-vs-module distinction, error recovery, and pagination.
var instructions = doc("instructions.md")

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
		Description: doc("search.md"),
		Annotations: readOnly(),
	}, s.search)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_package",
		Description: doc("get_package.md"),
		Annotations: readOnly(),
	}, s.getPackage)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_package_symbols",
		Description: doc("get_package_symbols.md"),
		Annotations: readOnly(),
	}, s.getSymbols)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_imported_by",
		Description: doc("get_imported_by.md"),
		Annotations: readOnly(),
	}, s.getImportedBy)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_module",
		Description: doc("get_module.md"),
		Annotations: readOnly(),
	}, s.getModule)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "list_module_versions",
		Description: doc("list_module_versions.md"),
		Annotations: readOnly(),
	}, s.listVersions)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "list_module_packages",
		Description: doc("list_module_packages.md"),
		Annotations: readOnly(),
	}, s.listPackages)

	mcp.AddTool(m, &mcp.Tool{
		Name:        "get_vulnerabilities",
		Description: doc("get_vulnerabilities.md"),
		Annotations: readOnly(),
	}, s.getVulns)
}
