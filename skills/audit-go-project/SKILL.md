---
name: audit-go-project
description: Audit a Go project's dependencies for known vulnerabilities, unmaintained modules, and unused requirements. Use when asked to security-scan, vuln-check, dependency-audit, or health-check a Go module and its dependency tree. Relies on the pkgsite MCP tools (get_vulnerabilities, get_module, list_module_versions).
---

Audit the Go project (the path the user gives, or the current directory) and all of its dependencies — including test-only dependencies, which run on CI — for security and maintenance risk. The pkgsite MCP tools (get_vulnerabilities, get_module, list_module_versions) are the primary source; govulncheck, when available, is an additional reachability check, not the main scanner.

1. Enumerate the modules and their selected versions with `go list -m all` (it covers build and test dependencies); if no Go toolchain is available, read go.mod and go.sum. Also run `go list -deps -test ./...` to learn which modules are actually imported (by build or test code) versus only present in the graph.
2. Known vulnerabilities: for each module at its selected version, call get_vulnerabilities to list advisories from the Go vulnerability database. A "not found" (404) means that version could not be checked — common for pseudo-versions (untagged `v0.0.0-<commit>`) — so treat it as unknown, not clean. For each advisory, find the fixed version; if get_vulnerabilities omits it, call list_module_versions and re-check newer versions for the first clean one. Additionally, if govulncheck is available, run `govulncheck -test ./...` to see which advisories are reachable from the code or tests (`-test` covers test code and test-only deps; if it fails to load because the tests do not compile, retry without `-test` and say so). Use its reachability as extra signal on top of the pkgsite results, not as a replacement.
3. Unmaintained modules: call get_module for each module to read its latest version and commit time. Flag modules whose latest release is old (roughly two years or more) or several releases behind the version in use. `go list -m -u all` also lists available updates and any author-deprecated module — report deprecated modules prominently, as that is the clearest "no longer supported" signal.
4. Unused modules: prefer `go mod tidy -diff` — the require entries it would remove are the genuinely unused ones, since it keeps modules still needed transitively. If that is unavailable, approximate by flagging modules in the graph that no package imports, but note that over-reports legitimate transitive dependencies. Recommend `go mod tidy` to drop the unused requirements.

Then report, in this order:
- A one-line summary, e.g. "Scanned 40 modules: 2 vulnerable, 3 unmaintained, 5 unused."
- Vulnerabilities: a table of only the affected modules (module, version, advisory ID, summary, fixed version, and — if govulncheck ran — whether the code or tests reach it). Skip if none.
- Unmaintained: a short table (module, version, latest, latest release date, reason: deprecated / stale / behind). Skip if none.
- Unused: a list of modules removable with `go mod tidy`. Skip if none.
- The minimal upgrades and removals you recommend, prioritizing reachable vulnerabilities.

If govulncheck was not available, note that vulnerability results reflect advisories by module and version only, not reachability, and do not cover the Go standard library or toolchain.
