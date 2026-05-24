Scan the Go project{{if .Path}} at {{.Path}}{{else}} in the current directory{{end}} and all of its dependencies for known vulnerabilities:
1. Enumerate the project's modules and their selected versions: prefer "go list -m all" if a Go toolchain is available, otherwise read go.mod and go.sum{{if .Path}} under {{.Path}}{{end}}.
2. For each module at its selected version, call get_vulnerabilities to list advisories from the Go vulnerability database.
3. For every advisory found, identify the version that fixes it: if get_vulnerabilities omits the fixed version, call list_module_versions and re-check newer versions to find the first clean one.
4. If a Go toolchain with govulncheck is available, also run "govulncheck ./..."{{if .Path}} in {{.Path}}{{end}} for the call-graph reachability analysis the pkg.go.dev data cannot provide. Distinguish advisories it reports as actually called (symbol or package results) from those merely present in a required module (module results); govulncheck also reports each advisory's fixed version, affected platforms, and any standard-library advisories.
Then report, in this order:
- A one-line summary of the scan, e.g. "Scanned 10 modules; 1 has known vulnerabilities."
- A table listing only the affected modules (module, version, advisory ID, summary, fixed version, and — if govulncheck ran — whether your code actually calls it). Omit modules with no known vulnerabilities so the table stays small; if none are affected, say so and skip the table entirely.
- The minimal upgrades you recommend, prioritizing any advisory govulncheck reports as reachable.
If govulncheck was not available, note that these results reflect advisories by module and version only, not whether the vulnerable code is actually reachable, and do not cover the Go standard library or toolchain.
