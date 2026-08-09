// Package server wires the pkgsite client to an MCP server, exposing each
// pkg.go.dev API endpoint as a tool.
package server

import (
	"context"
	"embed"
	"strings"
	"time"

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

// Display metadata advertised alongside the server name and version, so hosts
// have something friendlier than the binary name to show a user.
const (
	title       = "Go Package Index (pkg.go.dev)"
	description = "Search pkg.go.dev and inspect Go modules, packages, symbols, importers, and vulnerabilities."
	websiteURL  = "https://github.com/sv-tools/pkgsite-mcp"
)

// New returns an MCP server that exposes the pkg.go.dev API as tools, backed by
// the given client.
func New(client *pkgsite.Client, name, version string) *mcp.Server {
	s := &Server{client: client}
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:        name,
			Title:       title,
			Description: description,
			Version:     version,
			WebsiteURL:  websiteURL,
		},
		&mcp.ServerOptions{Instructions: instructions},
	)
	mcpServer.AddReceivingMiddleware(cacheableLists)
	s.registerTools(mcpServer)
	s.registerPrompts(mcpServer)
	return mcpServer
}

// listTTL is the freshness hint advertised on tools/list and prompts/list
// results (SEP-2549, added in go-sdk v1.7.0). Both sets are registered once in
// New and never change while the process runs, so a client can safely reuse a
// listing instead of re-fetching it; the SDK's default of zero would mark every
// listing immediately stale. The hint is deliberately finite so that a client
// holding a listing across a server upgrade picks up the new one before long.
const listTTL = time.Hour

// cacheableLists sets the SEP-2549 ttlMs hint on the list results that the SDK
// leaves at zero. It is receiving middleware because these results answer
// client-to-server requests; sending middleware only sees requests the server
// itself initiates.
func cacheableLists(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		res, err := next(ctx, method, req)
		switch r := res.(type) {
		case *mcp.ListToolsResult:
			r.TTLMs = int(listTTL / time.Millisecond)
		case *mcp.ListPromptsResult:
			r.TTLMs = int(listTTL / time.Millisecond)
		}
		return res, err
	}
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
