# pkgsite-mcp

An [MCP](https://modelcontextprotocol.io) server that exposes the
[pkg.go.dev](https://pkg.go.dev) API as tools, so an LLM can search for Go
packages and inspect modules, packages, symbols, importers, and
vulnerabilities — without downloading anything.

It wraps the pkg.go.dev v1beta HTTP API (see
[the announcement](https://go.dev/blog/pkgsite-api)), the same API used by the
`pkgsite-cli` reference client. The server speaks MCP over **stdio**, the
transport used by Claude Code, Claude Desktop, and most IDE integrations.

## Install

```sh
go install github.com/sv-tools/pkgsite-mcp@latest
```

This installs the `pkgsite-mcp` binary into `$(go env GOPATH)/bin`.

## Use with Claude Code

```sh
claude mcp add pkgsite -- pkgsite-mcp
```

## Use with Claude Desktop / other MCP clients

Add the server to your client's MCP config (for Claude Desktop,
`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pkgsite": {
      "command": "pkgsite-mcp"
    }
  }
}
```

Use the absolute path to the binary if it is not on your `PATH`.

## Tools

| Tool                   | Description                                                                                      |
|------------------------|--------------------------------------------------------------------------------------------------|
| `search`               | Search pkg.go.dev for packages by name or keywords.                                              |
| `get_package`          | Package details: module, version, synopsis, and optionally rendered docs, imports, and licenses. |
| `get_package_symbols`  | Exported symbols (functions, types, methods, vars, consts) of a package.                         |
| `get_imported_by`      | Packages from other modules that import a given package (reverse deps; same-module excluded).    |
| `get_module`           | Module details: version, commit time, repo URL, and optionally README and licenses.              |
| `list_module_versions` | Available versions of a module.                                                                  |
| `list_module_packages` | Packages contained in a module at a version.                                                     |
| `get_vulnerabilities`  | Known vulnerabilities (Go vuln database) for a module path and version.                          |

Endpoints that can return many results (`search`, `get_package_symbols`,
`get_imported_by`, `list_module_versions`, `list_module_packages`,
`get_vulnerabilities`) accept an optional `limit` and a `token`. When a response
includes a non-empty `nextPageToken`, pass it back as `token` to fetch the next
page.

## Flags

| Flag       | Default              | Description                 |
|------------|----------------------|-----------------------------|
| `-server`  | `https://pkg.go.dev` | pkg.go.dev API server URL.  |
| `-timeout` | `30s`                | HTTP request timeout.       |
| `-version` |                      | Print the version and exit. |

## Development

```sh
go build ./...
go test ./...
go vet ./...
```

Layout:

- `internal/pkgsite` — a small, dependency-free client for the pkg.go.dev
  v1beta API.
- `internal/server` — registers each API endpoint as an MCP tool.
- `main.go` — CLI entrypoint; runs the server over stdio.

## License

See [LICENSE](LICENSE).
