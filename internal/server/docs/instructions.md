Tools for querying pkg.go.dev, the Go package index. Every tool is read-only and
queries the live pkg.go.dev API.

Key distinctions and tips:

- A "package" is an import path (e.g. `net/http`, `github.com/google/uuid`); a
  "module" is its unit of versioning and release. Use `get_package` and
  `get_package_symbols` for packages, and `get_module`, `list_module_versions`,
  and `list_module_packages` for modules.
- To explore an unfamiliar module: `get_module` for metadata,
  `list_module_packages` for its packages, then `get_package` or
  `get_package_symbols` for a specific import path.
- If a package path is ambiguous, the call fails with a list of candidate
  modules; retry with the `module` argument set to one of them.
- Paginated tools return a `nextPageToken`; pass it back as `token` to fetch the
  next page. Results are capped by `limit` (a server-side default applies when
  it is omitted) — raise it only when you need more.
- When requesting documentation, prefer `doc=md` or `doc=text`; `doc=html` is far
  more verbose. README contents, full license texts, and import lists can be
  large, so request them only when needed.
- `get_vulnerabilities` checks the Go vulnerability database; an empty result
  means no known vulnerabilities for that module version.
