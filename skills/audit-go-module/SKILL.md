---
name: audit-go-module
description: Check a single Go module for known vulnerabilities and whether it is up to date. Use when asked to audit, vuln-check, or assess one module path (optionally at a version). Relies on the pkgsite MCP tools (get_vulnerabilities, get_module, list_module_versions).
---

Audit the Go module the user names (optionally at a given version; default to the latest) for supply-chain risk:
1. Call get_vulnerabilities for the module (at the version, if given) to list known advisories from the Go vulnerability database.
2. Call get_module and list_module_versions to check whether that version is the latest and when it was released.
3. For each vulnerability, report its ID, summary, and the version that fixes it.

Then state whether the module is safe to use and recommend an upgrade if one is warranted.
