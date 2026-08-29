// Package pkgsite provides a small client for the pkg.go.dev v1 HTTP API.
//
// The API is stateless and GET-only. See https://go.dev/blog/pkgsite-api for
// background. This client mirrors the surface of the reference client that
// ships with golang.org/x/pkgsite (cmd/internal/pkgsite-cli/client), but is an
// independent, dependency-free reimplementation.
package pkgsite

import (
	"fmt"
	"strings"
	"time"
)

// PaginatedResponse is a generic paginated response. A non-empty NextPageToken
// indicates more results are available; pass it back as the token option to
// fetch the next page.
type PaginatedResponse[T any] struct {
	Items         []T    `json:"items"`
	Total         int    `json:"total"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// PackageInfo holds the fields common to a package across responses.
type PackageInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Synopsis string `json:"synopsis"`
	// IsRedistributable reports whether the license allows distribution.
	IsRedistributable bool `json:"isRedistributable"`
}

// Package is the response for /v1/package/{packagePath}.
type Package struct {
	ModulePath        string    `json:"modulePath"`
	Version           string    `json:"version"`
	IsLatest          bool      `json:"isLatest"`
	IsStandardLibrary bool      `json:"isStandardLibrary"`
	GOOS              string    `json:"goos"`
	GOARCH            string    `json:"goarch"`
	Docs              string    `json:"docs,omitempty"`
	Imports           []string  `json:"imports,omitempty"`
	Licenses          []License `json:"licenses,omitempty"`
	PackageInfo
}

// PackagesResponse is the response for /v1/packages/{modulePath}.
type PackagesResponse struct {
	ModulePath        string                         `json:"modulePath"`
	Version           string                         `json:"version"`
	IsStandardLibrary bool                           `json:"isStandardLibrary"`
	Packages          PaginatedResponse[PackageInfo] `json:"packages"`
}

// ModulePackageResponse is a single package listed for a module.
type ModulePackageResponse struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Synopsis string `json:"synopsis"`
}

// License is license information in API responses.
type License struct {
	Types    []string `json:"types"`
	FilePath string   `json:"filePath"`
	Contents string   `json:"contents,omitempty"`
}

// PackageImportedBy is the response for /v1/imported-by/{packagePath}.
type PackageImportedBy struct {
	ModulePath string                    `json:"modulePath"`
	Version    string                    `json:"version"`
	ImportedBy PaginatedResponse[string] `json:"importedBy"`
}

// ModuleVersion is a single version from /v1/versions/{modulePath}.
type ModuleVersion struct {
	ModulePath string `json:"modulePath"`
	Version    string `json:"version"`
	// CommitTime is the time the version was created, as reported by the
	// module proxy's .info endpoint.
	CommitTime time.Time `json:"commitTime"`
	// IsRedistributable reports whether the license allows distribution.
	IsRedistributable bool `json:"isRedistributable"`
	// HasGoMod reports whether the version has a go.mod file.
	HasGoMod bool `json:"hasGoMod"`
	// LatestVersion is the module's latest unretracted version, repeated on
	// every item so a single call shows whether a version is current.
	LatestVersion string `json:"latestVersion"`
	// Deprecated reports whether the module is currently deprecated, as declared
	// by the latest version's go.mod; DeprecationReason is the text of that
	// "// Deprecated:" comment, when the author gave one. This is a module-wide
	// status repeated on every item, not per-version history.
	Deprecated        bool   `json:"deprecated"`
	DeprecationReason string `json:"deprecationReason,omitempty"`
	// Retracted reports whether this specific version is named by a retract
	// directive in the latest version's go.mod; RetractionReason is the
	// rationale, when the author gave one. A retracted version should not be
	// selected.
	Retracted        bool   `json:"retracted"`
	RetractionReason string `json:"retractionReason,omitempty"`
}

// Module is the response for /v1/module/{modulePath}.
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	// CommitTime is the time the version was created, as reported by the
	// module proxy's .info endpoint.
	CommitTime        time.Time `json:"commitTime"`
	IsLatest          bool      `json:"isLatest"`
	IsRedistributable bool      `json:"isRedistributable"`
	IsStandardLibrary bool      `json:"isStandardLibrary"`
	HasGoMod          bool      `json:"hasGoMod"`
	RepoURL           string    `json:"repoUrl"`
	GoModContents     string    `json:"goModContents,omitempty"`
	Readme            *Readme   `json:"readme,omitempty"`
	Licenses          []License `json:"licenses,omitempty"`
}

// Readme is a README file.
type Readme struct {
	Filepath string `json:"filepath"`
	Contents string `json:"contents"`
}

// Symbol is an exported symbol in a package.
type Symbol struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Synopsis string `json:"synopsis"`
	Parent   string `json:"parent,omitempty"`
}

// PackageSymbols is the response for /v1/symbols/{packagePath}.
type PackageSymbols struct {
	ModulePath string                    `json:"modulePath"`
	Version    string                    `json:"version"`
	Symbols    PaginatedResponse[Symbol] `json:"symbols"`
}

// SearchResult is a single result from /v1/search.
type SearchResult struct {
	PackagePath string `json:"packagePath"`
	ModulePath  string `json:"modulePath"`
	Version     string `json:"version"`
	Synopsis    string `json:"synopsis"`
}

// Vulnerability is a vulnerability report for a module path and version.
type Vulnerability struct {
	ID           string `json:"id"`
	Summary      string `json:"summary"`
	Details      string `json:"details"`
	FixedVersion string `json:"fixedVersion"`
}

// Candidate is a possible resolution for an ambiguous package path.
type Candidate struct {
	ModulePath  string `json:"modulePath"`
	PackagePath string `json:"packagePath"`
}

// APIError is a structured error returned by the API for a non-200 response.
// It implements error.
type APIError struct {
	Code       int         `json:"code"` // HTTP status code
	Message    string      `json:"message"`
	Fixes      []string    `json:"fixes,omitempty"`      // suggestions for how to fix
	Candidates []Candidate `json:"candidates,omitempty"` // for ambiguous paths
}

func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString(e.Message)
	if e.Code >= 100 {
		fmt.Fprintf(&b, " (HTTP %d)", e.Code)
	}
	if len(e.Candidates) > 0 {
		b.WriteString("; specify a module path, one of:")
		for _, c := range e.Candidates {
			fmt.Fprintf(&b, "\n  - %s", c.ModulePath)
		}
	}
	if len(e.Fixes) > 0 {
		b.WriteString("\nTo fix:")
		for _, f := range e.Fixes {
			fmt.Fprintf(&b, "\n  - %s", f)
		}
	}
	return b.String()
}
