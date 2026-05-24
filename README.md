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

## Prompts

The server also exposes guided, multi-tool workflows as MCP prompts:

| Prompt          | Arguments           | Description                                                                                           |
|-----------------|---------------------|-------------------------------------------------------------------------------------------------------|
| `audit_module`  | `module`, `version` | Check a module for known vulnerabilities and whether it is up to date.                                |
| `audit_project` | `path`              | Audit a Go project's dependencies for vulnerabilities, unmaintained modules, and unused requirements. |
| `find_package`  | `need`              | Search for packages that meet a need and recommend one.                                               |

## Flags

| Flag         | Default              | Description                                      |
|--------------|----------------------|--------------------------------------------------|
| `-server`    | `https://pkg.go.dev` | pkg.go.dev API server URL.                       |
| `-timeout`   | `30s`                | HTTP request timeout.                            |
| `-retries`   | `2`                  | Retry attempts for transient failures (429/5xx). |
| `-cache-ttl` | `5m`                 | Response cache TTL; `0` disables caching.        |
| `-version`   |                      | Print the version and exit.                      |

Transient failures (network errors and HTTP 429/5xx) are retried with
exponential backoff, honoring any `Retry-After` header. Successful responses are
cached in memory for `-cache-ttl`, keyed by request URL, to avoid refetching
during multi-step exploration.

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
- `internal/server/docs` — embedded Markdown for the tool and prompt
  descriptions and the server instructions, so the prose can be edited without
  touching Go.
- `main.go` — CLI entrypoint; runs the server over stdio.

## License

See [LICENSE](LICENSE).
