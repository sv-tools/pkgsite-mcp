Audit the Go module {{.Module}}{{if .Version}}@{{.Version}}{{end}} for supply-chain risk:
1. Call get_vulnerabilities for {{printf "%q" .Module}}{{if .Version}} at version {{.Version}}{{end}} to list known advisories from the Go vulnerability database.
2. Call get_module and list_module_versions to check whether that version is the latest and when it was released.
3. For each vulnerability, report its ID, summary, and the version that fixes it.
Then state whether the module is safe to use and recommend an upgrade if one is warranted.
