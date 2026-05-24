---
name: find-go-package
description: Find a Go package that meets a stated need and recommend one. Use when asked to discover, search for, or pick a Go library for a task (e.g. "parse YAML", "HTTP router"). Relies on the pkgsite MCP tools (search, get_package, get_imported_by).
---

Find a Go package that meets the user's stated need:
1. Call search with relevant keywords.
2. For the most promising results, call get_package to compare synopsis, latest version, and license.
3. Optionally call get_imported_by to gauge real-world adoption.

Recommend one package and explain why, giving its import path and a short usage note.
